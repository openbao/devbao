package bao

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
)

const NODE_JSON_NAME = "node.json"
const INSTANCE_CONFIG_NAME = "config.hcl"

type Node struct {
	Name string `json:"name"`
	Type string `json:"type"`

	Exec   *ExecEnvironment `json:"exec"`
	Config NodeConfig       `json:"config"`
}

type NodeConfigOpt interface{}

var _ NodeConfigOpt = Listener(nil)
var _ NodeConfigOpt = Storage(nil)
var _ NodeConfigOpt = &DevConfig{}

func LoadNode(name string) (*Node, error) {
	var node Node
	node.Name = name

	if err := node.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to read node (%v) configuration: %w", name, err)
	}

	if err := node.Validate(); err != nil {
		return nil, fmt.Errorf("invalid node (%v) configuration: %w", name, err)
	}

	return &node, nil
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
		case Storage:
			n.Config.Storage = tOpt
		case *DevConfig:
			n.Config.Dev = tOpt
		default:
			return nil, fmt.Errorf("unknown type of node configuration option at index %d: %v (%T)", index, opt, opt)
		}
	}

	if err := n.Validate(); err != nil {
		return nil, fmt.Errorf("invalid node configuration: %w", err)
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

	return n.Config.Validate()
}

func (n *Node) GetDirectory() string {
	usr, _ := user.Current()
	dir := usr.HomeDir

	return filepath.Join(dir, ".local/share/devbao/nodes", n.Name)
}

func (n *Node) buildExec() error {
	if err := n.Validate(); err != nil {
		return fmt.Errorf("failed to validate node definition: %w", err)
	}

	directory := n.GetDirectory()
	if err := os.MkdirAll(directory, 0755); err != nil {
		return fmt.Errorf("failed to create node directory (%v): %w", directory, err)
	}

	addr, _, err := n.Config.GetConnectAddr()
	if err != nil {
		return fmt.Errorf("failed to get connection address for node %v: %w", n.Name, err)
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

	if err := n.Clean(); err != nil {
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

		return disk.Exec.Kill()
	}

	return n.Exec.Kill()
}

func (n *Node) Clean() error {
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

	// Now, persist node configuration.
	return n.SaveConfig()
}

func (n *Node) LoadConfig() error {
	directory := n.GetDirectory()
	path := filepath.Join(directory, NODE_JSON_NAME)
	configFile, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open config file (`%v`) for reading: %w", path, err)
	}

	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(n); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

func (n *Node) SaveConfig() error {
	directory := n.GetDirectory()
	path := filepath.Join(directory, NODE_JSON_NAME)
	configFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
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
	path := filepath.Join(directory, INSTANCE_CONFIG_NAME)

	configFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open instance config file (`%v`) for writing: %w", path, err)
	}

	defer configFile.Close()

	if _, err := io.WriteString(configFile, config); err != nil {
		return "", fmt.Errorf("failed to write instance config file (`%v`): %w", path, err)
	}

	return path, nil
}
