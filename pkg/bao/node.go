package bao

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/openbao/openbao/api/v2"
)

const (
	NodeJsonName       = "node.json"
	InstanceConfigName = "config.hcl"
)

type Node struct {
	Name string `json:"name"`
	Type string `json:"type"`

	Exec   *ExecEnvironment `json:"exec"`
	Config NodeConfig       `json:"config"`

	Addr       string   `json:"addr"`
	Ca         string   `json:"ca"`
	Token      string   `json:"token"`
	UnsealKeys []string `json:"unseal_keys,omitempty"`

	Cluster  string `json:"cluster,omitempty"`
	NonVoter bool   `json:"non_voter"`
}

func (n *Node) FromInterface(iface map[string]interface{}) error {
	n.Name = iface["name"].(string)
	n.Type = iface["type"].(string)

	if _, present := iface["addr"]; present {
		n.Addr = iface["addr"].(string)
	}

	if _, present := iface["ca"]; present {
		n.Ca = iface["ca"].(string)
	}

	if _, present := iface["token"]; present {
		n.Token = iface["token"].(string)
	}

	if _, present := iface["cluster"]; present {
		n.Cluster = iface["cluster"].(string)
	}

	if unsealKeysRaw, ok := iface["unseal_keys"].([]interface{}); ok {
		n.UnsealKeys = nil
		for _, keyRaw := range unsealKeysRaw {
			n.UnsealKeys = append(n.UnsealKeys, keyRaw.(string))
		}
	}

	data, present := iface["exec"]
	if present {
		j, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("error re-marshalling exec config: %w", err)
		}

		if err := json.Unmarshal(j, &n.Exec); err != nil {
			return fmt.Errorf("error unmarshaling exec config; %w", err)
		}
	}

	return n.Config.FromInterface(iface["config"].(map[string]interface{}))
}

type NodeConfigOpt interface{}

var (
	_ NodeConfigOpt = Listener(nil)
	_ NodeConfigOpt = Storage(nil)
	_ NodeConfigOpt = Seal(nil)
	_ NodeConfigOpt = &DevConfig{}
	_ NodeConfigOpt = Audit(nil)
)

func ListNodes() ([]string, error) {
	dir := NodeBaseDirectory()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create node directory (%v): %w", dir, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("error listing node directory (`%v`): %w", dir, err)
	}

	var results []string
	for _, entry := range entries {
		if entry.IsDir() {
			results = append(results, entry.Name())
		}
	}

	return results, nil
}

func NodeExists(name string) (bool, error) {
	nodes, err := ListNodes()
	if err != nil {
		return false, err
	}

	for _, node := range nodes {
		if node == name {
			return true, nil
		}
	}

	return false, nil
}

func LoadNodeUnvalidated(name string) (*Node, error) {
	var node Node
	node.Name = name

	if err := node.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to read node (%v) configuration: %w", name, err)
	}

	return &node, nil
}

func LoadNode(name string) (*Node, error) {
	node, err := LoadNodeUnvalidated(name)
	if err != nil {
		return nil, err
	}

	if err := node.Validate(); err != nil {
		return nil, fmt.Errorf("invalid node (%v) configuration: %w", name, err)
	}

	return node, nil
}

func BuildNode(name string, product string, opts ...NodeConfigOpt) (*Node, error) {
	n := &Node{
		Name: name,
		Type: product,
	}

	for index, opt := range opts {
		switch tOpt := opt.(type) {
		case Listener:
			n.Config.Listeners = append(n.Config.Listeners, tOpt)
		case Seal:
			n.Config.Seals = append(n.Config.Seals, tOpt)
		case Storage:
			n.Config.Storage = tOpt
		case *DevConfig:
			n.Config.Dev = tOpt
		case Audit:
			n.Config.Audits = append(n.Config.Audits, tOpt)
		default:
			return nil, fmt.Errorf("unknown type of node configuration option at index %d: %v (%T)", index, opt, opt)
		}
	}

	if err := n.SaveConfig(); err != nil {
		return nil, fmt.Errorf("failed saving initial configuration: %w", err)
	}

	return n, nil
}

func (n *Node) Validate() error {
	if n.Name == "" {
		if n.Config.Dev == nil {
			n.Name = "dev"
		} else {
			n.Name = "prod"
		}
	}

	if n.Type != "" && n.Type != "bao" && n.Type != "vault" {
		return fmt.Errorf("invalid node type (`%s`): expected either empty (``), OpenBao (`bao`), or HashiCorp Vault (`vault`)", n.Type)
	}

	if n.Config.Dev != nil && n.Token == "" && n.Config.Dev.Token != "" {
		n.Token = n.Config.Dev.Token
	}

	for index, key := range n.UnsealKeys {
		if key == "" {
			return fmt.Errorf("blank unseal key at index %d", index)
		}
	}

	if n.Exec != nil && n.Addr != "" {
		// Address is a URL scheme; we wish to know what the underlying
		// connection address should be.
		url, err := url.Parse(n.Addr)
		if err != nil {
			return fmt.Errorf("failed to parse address (`%v`): %w", n.Addr, err)
		}

		n.Exec.ConnectAddress = url.Host
	}

	return n.Config.Validate()
}

func NodeBaseDirectory() string {
	usr, _ := user.Current()
	dir := usr.HomeDir

	return filepath.Join(dir, ".local/share/devbao/nodes")
}

func (n *Node) GetDirectory() string {
	return filepath.Join(NodeBaseDirectory(), n.Name)
}

func (n *Node) buildExec() error {
	if err := n.Validate(); err != nil {
		return fmt.Errorf("failed to validate node definition: %w", err)
	}

	directory := n.GetDirectory()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("failed to create node directory (%v): %w", directory, err)
	}

	addr, _, _, err := n.Config.GetConnectAddr(directory)
	if err != nil {
		return fmt.Errorf("failed to get connection address for node %v: %w", n.Name, err)
	}

	if n.Addr != "" {
		addr = n.Exec.ConnectAddress
	}

	args, err := n.Config.AddArgs(directory)
	if err != nil {
		return fmt.Errorf("failed to build arguments to binary: %w", err)
	}

	config, err := n.Config.ToConfig(directory)
	if err == nil && config == "" && n.Config.Dev == nil {
		err = fmt.Errorf("expected non-dev server to have non-empty configuration; are listeners or storage missing")
	}
	if err != nil {
		return fmt.Errorf("failed to build node's configuration (%s): %w", n.Name, err)
	}

	if config != "" {
		path, err := n.SaveInstanceConfig(config)
		if err != nil {
			return fmt.Errorf("error persisting node configuration: %w", err)
		}

		args = append(args, "-config="+path)
	}

	n.Exec = &ExecEnvironment{
		Args:      args,
		Directory: directory,

		ConnectAddress: addr,
	}

	return nil
}

func (n *Node) Start() error {
	_ = n.Kill()

	if err := n.Clean(false); err != nil {
		return fmt.Errorf("failed to clean up existing node: %w", err)
	}

	return n.Resume()
}

func (n *Node) Kill() error {
	if n.Exec == nil || n.Exec.Pid == 0 {
		disk, err := LoadNode(n.Name)
		if err != nil {
			return fmt.Errorf("error loading node from disk while killing: %w", err)
		}

		if disk.Exec == nil {
			return fmt.Errorf("node has no execution state: %w", err)
		}

		return disk.Exec.Kill()
	}

	return n.Exec.Kill()
}

func (n *Node) Clean(force bool) error {
	if n.Cluster != "" {
		cluster, err := LoadCluster(n.Cluster)
		if err != nil && !force {
			return fmt.Errorf("failed to load node's cluster (`%v`): %w", n.Cluster, err)
		}

		if cluster != nil {
			if err := cluster.RemoveNodeHACluster(n); err != nil && !force {
				return fmt.Errorf("failed to remove node (`%v`) from cluster (`%v`): %w", n.Name, cluster.Name, err)
			}
		}
	}

	directory := n.GetDirectory()
	return os.RemoveAll(directory)
}

func (n *Node) Resume() error {
	if err := n.buildExec(); err != nil {
		return fmt.Errorf("failed to build execution environment: %w", err)
	}

	var err error
	switch n.Type {
	case "":
		err = Exec(n.Exec)
	case "bao":
		err = ExecBao(n.Exec)
	case "vault":
		err = ExecVault(n.Exec)
	default:
		err = fmt.Errorf("unknown execution type: `%s`", n.Type)
	}

	if err != nil {
		return err
	}

	// Update node configuration.
	return n.SaveConfig()
}

func (n *Node) LoadConfig() error {
	directory := n.GetDirectory()
	path := filepath.Join(directory, NodeJsonName)
	configFile, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open config file (`%v`) for reading: %w", path, err)
	}

	defer configFile.Close()

	// We need to unmarshal to an intermediate interface so that we can figure
	// out the correct types for the Storage and Listeners.
	var cfg map[string]interface{}

	if err := json.NewDecoder(configFile).Decode(&cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := n.FromInterface(cfg); err != nil {
		return fmt.Errorf("failed to translate config: %w", err)
	}

	return nil
}

func (n *Node) SaveConfig() error {
	if err := n.Validate(); err != nil {
		return fmt.Errorf("failed validating config prior to saving: %w", err)
	}

	directory := n.GetDirectory()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("failed to create node directory (%v): %w", directory, err)
	}

	path := filepath.Join(directory, NodeJsonName)
	configFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open config file (`%v`) for writing: %w", path, err)
	}

	defer configFile.Close()

	if err := json.NewEncoder(configFile).Encode(n); err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return nil
}

func (n *Node) SaveInstanceConfig(config string) (string, error) {
	directory := n.GetDirectory()
	path := filepath.Join(directory, InstanceConfigName)

	configFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to open instance config file (`%v`) for writing: %w", path, err)
	}

	defer configFile.Close()

	if _, err := io.WriteString(configFile, config); err != nil {
		return "", fmt.Errorf("failed to write instance config file (`%v`): %w", path, err)
	}

	return path, nil
}

func (n *Node) GetConnectAddr() (string, string, error) {
	if n.Addr != "" && (!strings.HasPrefix(n.Addr, "https") || n.Ca != "") {
		return n.Addr, n.Ca, nil
	}

	directory := n.GetDirectory()
	addr, isTls, ca, err := n.Config.GetConnectAddr(directory)
	if err != nil {
		return "", "", fmt.Errorf("failed to get connection address for node %v: %w", n.Name, err)
	}

	scheme := "http"
	if isTls {
		scheme = "https"
	}

	if strings.HasPrefix(n.Addr, "https") && n.Ca == "" {
		n.Ca = ca
	}

	return fmt.Sprintf("%v://%v", scheme, addr), ca, nil
}

func (n *Node) GetEnv() (map[string]string, error) {
	results := make(map[string]string)
	prefix := "VAULT_"

	addr, ca, err := n.GetConnectAddr()
	if err != nil {
		return nil, err
	}

	results[prefix+"ADDR"] = addr
	results[prefix+"TOKEN"] = n.Token

	if ca != "" {
		results[prefix+"CACERT"] = ca
	}

	return results, nil
}

func (n *Node) GetClient() (*api.Client, error) {
	addr, ca, err := n.GetConnectAddr()
	if err != nil {
		return nil, err
	}

	cfg := &api.Config{
		Address: addr,
	}
	if ca != "" {
		tls := &api.TLSConfig{
			CACert: ca,
		}

		if err := cfg.ConfigureTLS(tls); err != nil {
			return nil, fmt.Errorf("failed to configure tls for client: %w", err)
		}
	}
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	client.SetToken(n.Token)
	return client, nil
}

func (n *Node) SetToken(token string) (bool /* validated */, error) {
	n.Token = token

	if err := n.Exec.ValidateRunning(); err == nil {
		if err := n.ValidateToken(token); err != nil {
			return false, fmt.Errorf("token validation failed: %w", err)
		}

		return true, nil
	}

	return false, nil
}

func (n *Node) ValidateToken(token string) error {
	client, err := n.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client: %w", err)
	}

	client.SetToken(token)

	_, err = client.Sys().ListMounts()
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	return nil
}

func (n *Node) SetAddress(addr string) (bool, error) {
	n.Addr = addr

	if err := n.Exec.ValidateRunning(); err == nil {
		if err := n.ValidateAddress(); err != nil {
			return false, fmt.Errorf("address validation failed: %w", err)
		}

		return true, n.Validate()
	}

	return false, n.Validate()
}

func (n *Node) ValidateAddress() error {
	client, err := n.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client: %w", err)
	}

	_, err = client.Sys().InitStatus()
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	return nil
}

func (n *Node) Initialize() error {
	client, err := n.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client for node: %w", err)
	}

	initialized, err := client.Sys().InitStatus()
	if err != nil {
		return fmt.Errorf("failed to read initialization status of node: %w", err)
	}

	if initialized {
		return fmt.Errorf("node is already initialized")
	}

	if n.Token != "" || len(n.UnsealKeys) != 0 {
		return fmt.Errorf("refusing to overwrite existing token or unseal keys")
	}

	req := &api.InitRequest{
		SecretShares:    3,
		SecretThreshold: 2,
	}

	if len(n.Config.Seals) > 0 {
		req = &api.InitRequest{
			RecoveryShares:    3,
			RecoveryThreshold: 2,
		}
	}

	resp, err := client.Sys().Init(req)
	if err != nil {
		return fmt.Errorf("failed to initialize specified node: %w", err)
	}

	if len(resp.KeysB64) != 0 && len(resp.RecoveryKeysB64) != 0 {
		return fmt.Errorf("both unseal keys (%v) and recovery keys (%v) were provided in the response", resp.KeysB64, resp.RecoveryKeysB64)
	}

	if len(resp.KeysB64) != 0 {
		n.UnsealKeys = resp.KeysB64
	} else if len(resp.RecoveryKeysB64) != 0 {
		n.UnsealKeys = resp.RecoveryKeysB64
	}

	n.Token = resp.RootToken

	return n.SaveConfig()
}

func (n *Node) Unseal() (bool, error) {
	client, err := n.GetClient()
	if err != nil {
		return false, fmt.Errorf("failed to get client for node: %w", err)
	}

	if len(n.UnsealKeys) == 0 {
		return false, fmt.Errorf("no unseal keys stored for node %v", n.Name)
	}

	for index, key := range n.UnsealKeys {
		status, err := client.Sys().SealStatus()
		if err != nil {
			return false, fmt.Errorf("failed to fetch unseal status: %w", err)
		}

		if !status.Sealed {
			if index == 0 {
				return false, nil
			}
			break
		}

		_, err = client.Sys().Unseal(key)
		if err != nil {
			return false, fmt.Errorf("failed to provide unseal shard: %w", err)
		}
	}

	return true, nil
}

func (n *Node) PostInitializeUnseal() error {
	client, err := n.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client for node: %w", err)
	}

	directory := n.GetDirectory()

	for index, audit := range n.Config.Audits {
		if err := audit.PostUnseal(client, directory); err != nil {
			return fmt.Errorf("failed applying audit %v post-unseal: %w", index, err)
		}
	}

	return nil
}
