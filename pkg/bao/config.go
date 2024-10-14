package bao

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/openbao/openbao/api/v2"
)

type Unmarshable interface {
	FromInterface(iface map[string]interface{}) error
}

type ConfigBuilder interface {
	Unmarshable
	ToConfig(directory string) (string, error)
}

type ArgBuilder interface {
	AddArgs(directory string) ([]string, error)
}

type PostUnsealHook interface {
	PostUnseal(client *api.Client, directory string) error
}

const (
	TLS_CA_NAME    = "ca.pem"
	TLS_CERTS_NAME = "fullchain.pem"
	TLS_KEY_NAME   = "leaf-key.pem"
)

type TLSConfig struct {
	Certificates []string `json:"certs"`
	Key          string   `json:"key"`
}

func (t *TLSConfig) Write(caPath string, certPath string, keyPath string) error {
	caFile, err := os.OpenFile(caPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open cas to path (%v): %w", caPath, err)
	}
	defer caFile.Close()

	certFile, err := os.OpenFile(certPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open certs to path (%v): %w", certPath, err)
	}
	defer certFile.Close()

	for index, cert := range t.Certificates {
		if _, err := io.WriteString(certFile, strings.TrimSpace(cert)+"\n"); err != nil {
			return fmt.Errorf("failed to write cert %d to path (%v): %w", index, certPath, err)
		}

		// Write the last certificate as the CA certificate.
		if index == len(t.Certificates)-1 {
			if _, err := io.WriteString(caFile, strings.TrimSpace(cert)+"\n"); err != nil {
				return fmt.Errorf("failed to write cert %d to path (%v): %w", index, certPath, err)
			}
		}
	}

	keyFile, err := os.OpenFile(keyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open keys to path (%v): %w", keyPath, err)
	}
	defer keyFile.Close()

	if _, err := io.WriteString(keyFile, strings.TrimSpace(t.Key)+"\n"); err != nil {
		return fmt.Errorf("failed to write key to path (%v): %w", keyPath, err)
	}

	return nil
}

func (t *TLSConfig) GetCA() (*x509.Certificate, string, error) {
	if len(t.Certificates) == 1 {
		return nil, "", nil
	}

	caPem := t.Certificates[len(t.Certificates)-1]
	caBlock, rest := pem.Decode([]byte(caPem))
	if len(rest) != 0 {
		return nil, "", fmt.Errorf("unexpected trailing data after CA certificate: %v\n\nCA Certificate:\n%v", string(rest), caPem)
	}

	ca, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse CA certificate: %w\n\tCA Certificate:\n%v", err, caPem)
	}

	return ca, caPem, nil
}

type TCPListener struct {
	Address string     `json:"address"`
	TLS     *TLSConfig `json:"tls,omitempty"`
}

func (t *TCPListener) FromInterface(iface map[string]interface{}) error {
	t.Address = iface["address"].(string)
	if data, present := iface["tls"].(map[string]interface{}); present {
		t.TLS = &TLSConfig{}
		for _, certificate := range data["certs"].([]interface{}) {
			t.TLS.Certificates = append(t.TLS.Certificates, certificate.(string))
		}
		t.TLS.Key = data["key"].(string)
	}
	return nil
}

func (t *TCPListener) ToConfig(directory string) (string, error) {
	config := `listener "tcp" {` + "\n"

	if t.Address != "" {
		config += `  address = "` + t.Address + `"` + "\n"
	}

	if t.TLS == nil {
		config += `  tls_disable = true` + "\n"
	} else {
		caPath := filepath.Join(directory, TLS_CA_NAME)
		certPath := filepath.Join(directory, TLS_CERTS_NAME)
		keyPath := filepath.Join(directory, TLS_KEY_NAME)
		if err := t.TLS.Write(caPath, certPath, keyPath); err != nil {
			return "", fmt.Errorf("failed to persist TLS configuration: %w", err)
		}
	}

	config += "}\n"
	return config, nil
}

func getConnectionAddr(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("failed to parse address (`%v`): %w", address, err)
	}

	if host == "0.0.0.0" {
		// Connect over localhost instead as we can't connect to an wildcard
		// address as a client.
		host = "127.0.0.1"
	}

	return fmt.Sprintf("%v:%v", host, port), nil
}

func (t *TCPListener) GetConnectAddr(directory string) (string, string, error) {
	addr, err := getConnectionAddr(t.Address)
	if err != nil {
		return "", "", err
	}

	var ca string
	if t.TLS != nil {
		ca = filepath.Join(directory, TLS_CA_NAME)
	}

	return addr, ca, nil
}

type UnixListener struct {
	Path string `json:"path"`
}

func (u *UnixListener) FromInterface(iface map[string]interface{}) error {
	u.Path = iface["path"].(string)
	return nil
}

func (u *UnixListener) ToConfig(directory string) (string, error) {
	config := `listener "unix" {` + "\n"
	config += `path = "` + u.Path + `"`
	config += "\n}\n"
	return config, nil
}

func (u *UnixListener) GetConnectAddr(directory string) (string, string, error) {
	return "", "", fmt.Errorf("unix socket cannot be connected to via tcp")
}

type Listener interface {
	ConfigBuilder
	GetConnectAddr(string) (string, string, error)
}

var (
	_ Listener = &TCPListener{}
	_ Listener = &UnixListener{}
)

type RaftStorage struct{}

func (r *RaftStorage) FromInterface(iface map[string]interface{}) error {
	return nil
}

func (r *RaftStorage) ToConfig(directory string) (string, error) {
	path := filepath.Join(directory, "storage/raft")

	config := `storage "raft" {` + "\n"
	config += `  path = "` + path + `"` + "\n"
	config += "}\n"

	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("failed to make raft storage directory (%v): %w", path, err)
	}

	return config, nil
}

func (r *RaftStorage) StorageType() string {
	return "raft"
}

type PostgresqlStorage struct {
	ConnectionURL string `json:"connection_url"`
}

func (p *PostgresqlStorage) FromInterface(iface map[string]interface{}) error {
	p.ConnectionURL = iface["connection_url"].(string)
	return nil
}

func (p *PostgresqlStorage) ToConfig(directory string) (string, error) {
	path := filepath.Join(directory, "storage/postgresql")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("failed to make postgres storage directory (%v): %w", path, err)
	}

	// XXX - start Postgres container image and automatically volume-mount the data store.

	p.ConnectionURL = "postgres://openbao:Secret123@localhost:5432/openbao?sslmode=disable"

	config := `storage "postgresql" {` + "\n"
	config += `  connection_url = "` + p.ConnectionURL + `"` + "\n"
	config += `  table = "openbao_kv_store"` + "\n"
	config += `  ha_table = "openbao_ha_locks"` + "\n"
	config += `  ha_enabled = true` + "\n"
	config += "}\n"

	return config, nil
}

func (p *PostgresqlStorage) StorageType() string {
	return "postgresql"
}

type FileStorage struct{}

func (f *FileStorage) FromInterface(iface map[string]interface{}) error {
	return nil
}

func (f *FileStorage) ToConfig(directory string) (string, error) {
	path := filepath.Join(directory, "storage/file")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("failed to make file storage directory (%v): %w", path, err)
	}

	config := `storage "file" {` + "\n"
	config += `  path = "` + path + `"` + "\n"
	config += "}\n"

	return config, nil
}

func (f *FileStorage) StorageType() string {
	return "file"
}

type InmemStorage struct{}

func (i *InmemStorage) FromInterface(iface map[string]interface{}) error {
	return nil
}

func (i *InmemStorage) ToConfig(_ string) (string, error) {
	config := `storage "inmem" {}` + "\n"
	return config, nil
}

func (i *InmemStorage) StorageType() string {
	return "inmem"
}

type Storage interface {
	ConfigBuilder

	StorageType() string
}

var (
	_ Storage = &RaftStorage{}
	_ Storage = &FileStorage{}
	_ Storage = &InmemStorage{}
)

type Seal interface {
	ConfigBuilder

	UnsealHelper(client *api.Client) error
}

type TransitSeal struct {
	Address   string `json:"address"`
	Token     string `json:"token"`
	MountPath string `json:"mount_path"`
	KeyName   string `json:"key_name"`
	Disabled  bool   `json:"disabled"`
}

func (t *TransitSeal) UnsealHelper(client *api.Client) error { return nil }

func (t *TransitSeal) FromInterface(iface map[string]interface{}) error {
	t.Address = iface["address"].(string)
	t.Token = iface["token"].(string)
	t.MountPath = iface["mount_path"].(string)
	t.KeyName = iface["key_name"].(string)
	t.Disabled = iface["disabled"].(bool)
	return nil
}

func (t *TransitSeal) ToConfig(directory string) (string, error) {
	config := `seal "transit" {` + "\n"
	config += `  address = "` + t.Address + `"` + "\n"
	config += `  token = "` + t.Token + `"` + "\n"
	config += `  mount_path = "` + t.MountPath + `"` + "\n"
	config += `  key_name = "` + t.KeyName + `"` + "\n"
	config += `  disabled = ` + fmt.Sprintf("%v", t.Disabled) + "\n"
	config += "}\n"
	return config, nil
}

type CommonAudit struct {
	ConfigBuilder

	ElideListResponses bool   `json:"elide_list_responses"`
	Format             string `json:"format"`
	HmacAccessor       bool   `json:"hmac_accessor"`
	LogRaw             bool   `json:"log_raw"`
	Prefix             string `json:"prefix"`
}

func (c *CommonAudit) FromInterface(iface map[string]interface{}) error {
	c.ElideListResponses = iface["elide_list_responses"].(bool)
	c.Format = iface["format"].(string)
	c.HmacAccessor = iface["hmac_accessor"].(bool)
	c.LogRaw = iface["log_raw"].(bool)
	c.Prefix = iface["prefix"].(string)
	return nil
}

func (c *CommonAudit) ToConfig(directory string) (string, error) {
	// Audit does not go in the server HCL config.
	return "", nil
}

type FileAudit struct {
	CommonAudit

	FilePath string `json:"file_path"`
	Mode     string `json:"mode"`
}

func (f *FileAudit) FromInterface(iface map[string]interface{}) error {
	if err := f.CommonAudit.FromInterface(iface); err != nil {
		return fmt.Errorf("failed to parse common audit configuration: %w", err)
	}
	f.FilePath = iface["file_path"].(string)
	f.Mode = iface["mode"].(string)
	return nil
}

func (f *FileAudit) ToConfig(directory string) (string, error) {
	// Audit does not go in the server HCL config.
	return "", nil
}

func (f *FileAudit) PostUnseal(client *api.Client, directory string) error {
	name := "audit"
	if f.FilePath != "" {
		name += "-custom"
	}
	if f.Format != "" {
		name += "-" + f.Format
	}
	if f.LogRaw {
		name += "-raw"
	}

	filePath := filepath.Join(directory, name)
	filePath += ".log"

	if f.FilePath == "" {
		f.FilePath = filePath
	}

	opts := map[string]interface{}{
		"elide_list_responses": f.ElideListResponses,
		"hmac_accessor":        f.HmacAccessor,
		"log_raw":              f.LogRaw,
		"file_path":            f.FilePath,
	}
	if f.Format != "" {
		opts["format"] = f.Format
	}
	if f.Prefix != "" {
		opts["prefix"] = f.Prefix
	}
	if f.Mode != "" {
		opts["mode"] = f.Mode
	}

	resp, err := client.Logical().Read("sys/audit")
	if err != nil {
		return fmt.Errorf("failed to list audit devices; %w", err)
	}

	if _, present := resp.Data[name]; !present {
		data := map[string]interface{}{
			"type":    "file",
			"options": opts,
		}
		if _, err := client.Logical().Write("sys/audit/"+name, data); err != nil {
			return fmt.Errorf("failed to create audit device %v: %w", name, err)
		}
	}

	return nil
}

type Audit interface {
	ConfigBuilder
	PostUnsealHook
}

var _ Audit = &FileAudit{}

type NodeConfig struct {
	Dev           *DevConfig `json:"dev,omitempty"`
	ListenerTypes []string   `json:"listener_types,omitempty"`
	Listeners     []Listener `json:"listeners,omitempty"`
	StorageType   string     `json:"storage_type,omitempty"`
	Storage       Storage    `json:"storage,omitempty"`
	SealTypes     []string   `json:"seal_types,omitempty"`
	Seals         []Seal     `json:"seals,omitempty"`
	AuditTypes    []string   `json:"audit_types,omitempty"`
	Audits        []Audit    `json:"audits,omitempty"`
}

func (n *NodeConfig) FromInterface(iface map[string]interface{}) error {
	if data, present := iface["dev"]; present {
		n.Dev = &DevConfig{}
		if err := n.Dev.FromInterface(data.(map[string]interface{})); err != nil {
			return fmt.Errorf("failed to load dev config: %w", err)
		}
	}

	if listeners, present := iface["listener_types"]; present && listeners != nil {
		listenersDataRaw := iface["listeners"].([]interface{})
		listenerTypesRaw := listeners.([]interface{})
		if len(listenersDataRaw) != len(listenerTypesRaw) {
			return fmt.Errorf("unequal number of listener types (%v) as listeners (%v)", len(listenerTypesRaw), len(listenersDataRaw))
		}
		for index, listenerTypeRaw := range listenerTypesRaw {
			listenerType := listenerTypeRaw.(string)
			n.ListenerTypes = append(n.ListenerTypes, listenerType)

			switch listenerType {
			case "tcp":
				n.Listeners = append(n.Listeners, &TCPListener{})
			case "unix":
				n.Listeners = append(n.Listeners, &UnixListener{})
			default:
				return fmt.Errorf("unknown listener type at index %v: %v", index, listenerType)
			}

			listenerData := listenersDataRaw[index].(map[string]interface{})
			if err := n.Listeners[index].FromInterface(listenerData); err != nil {
				return fmt.Errorf("error parsing listener data at index %v: %v", index, err)
			}
		}
	}

	if _, present := iface["storage_type"]; present {
		n.StorageType = iface["storage_type"].(string)
		switch n.StorageType {
		case "raft":
			n.Storage = &RaftStorage{}
		case "postgresql":
			n.Storage = &PostgresqlStorage{}
		case "file":
			n.Storage = &FileStorage{}
		case "inmem":
			n.Storage = &InmemStorage{}
		case "":
		}

		if n.Storage != nil {
			if err := n.Storage.FromInterface(iface["storage"].(map[string]interface{})); err != nil {
				return fmt.Errorf("error parsing storage data: %w", err)
			}
		}
	}

	if seals, present := iface["seal_types"]; present && seals != nil {
		sealsDataRaw := iface["seals"].([]interface{})
		sealTypesRaw := seals.([]interface{})
		if len(sealsDataRaw) != len(sealTypesRaw) {
			return fmt.Errorf("unequal number of seal types (%v) as seals (%v)", len(sealTypesRaw), len(sealsDataRaw))
		}

		for index, sealTypeRaw := range sealTypesRaw {
			sealType := sealTypeRaw.(string)
			n.SealTypes = append(n.SealTypes, sealType)

			switch sealType {
			case "transit":
				n.Seals = append(n.Seals, &TransitSeal{})
			default:
				return fmt.Errorf("unknown seal type at index %v: %v", index, sealType)
			}

			sealData := sealsDataRaw[index].(map[string]interface{})
			if err := n.Seals[index].FromInterface(sealData); err != nil {
				return fmt.Errorf("error parsing seal data at index %v: %v", index, err)
			}
		}
	}

	if audits, present := iface["audit_types"]; present && audits != nil {
		auditsDataRaw := iface["audits"].([]interface{})
		auditTypesRaw := audits.([]interface{})
		if len(auditsDataRaw) != len(auditTypesRaw) {
			return fmt.Errorf("unequal number of audit types (%v) as audits (%v)", len(auditTypesRaw), len(auditsDataRaw))
		}

		for index, auditTypeRaw := range auditTypesRaw {
			auditType := auditTypeRaw.(string)
			n.AuditTypes = append(n.AuditTypes, auditType)

			switch auditType {
			case "file":
				n.Audits = append(n.Audits, &FileAudit{})
			default:
				return fmt.Errorf("unknown audit type at index %v: %v", index, auditType)
			}

			auditData := auditsDataRaw[index].(map[string]interface{})
			if err := n.Audits[index].FromInterface(auditData); err != nil {
				return fmt.Errorf("error parsing audit data at index %v: %v", index, err)
			}
		}
	}

	return nil
}

func (n *NodeConfig) Validate() error {
	if len(n.Listeners) == 0 && n.Dev == nil {
		return fmt.Errorf("no listeners specified and dev mode disabled")
	}

	n.ListenerTypes = nil
	for index, listener := range n.Listeners {
		switch listener.(type) {
		case *TCPListener:
			n.ListenerTypes = append(n.ListenerTypes, "tcp")
		case *UnixListener:
			n.ListenerTypes = append(n.ListenerTypes, "unix")
		default:
			return fmt.Errorf("unknown listenerType at index %v: %T / %v", index, listener, listener)
		}
	}

	if n.Dev == nil && n.Storage == nil {
		return fmt.Errorf("no storage specified and dev mode disabled")
	}

	switch n.Storage.(type) {
	case *RaftStorage:
		n.StorageType = "raft"
	case *PostgresqlStorage:
		n.StorageType = "postgresql"
	case *FileStorage:
		n.StorageType = "file"
	case *InmemStorage:
		n.StorageType = "inmem"
	case nil:
	default:
		return fmt.Errorf("unknown storage type: %T / %v", n.Storage, n.Storage)
	}

	n.SealTypes = nil
	for index, seal := range n.Seals {
		switch seal.(type) {
		case *TransitSeal:
			n.SealTypes = append(n.SealTypes, "transit")
		default:
			return fmt.Errorf("unknown seal type at index %v: %T / %v", index, seal, seal)
		}
	}

	n.AuditTypes = nil
	for index, audit := range n.Audits {
		switch audit.(type) {
		case *FileAudit:
			n.AuditTypes = append(n.AuditTypes, "file")
		default:
			return fmt.Errorf("unknown audit type at index %v: %T / %v", index, audit, audit)
		}
	}

	return nil
}

func (n *NodeConfig) ToConfig(directory string) (string, error) {
	if err := n.Validate(); err != nil {
		return "", err
	}

	config := ``
	for index, listener := range n.Listeners {
		lConfig, err := listener.ToConfig(directory)
		if err != nil {
			return "", fmt.Errorf("failed to build listener %d (%#v) to config: %w", index, listener, err)
		}

		config += lConfig + "\n"
	}

	// Check if the user has permissions to lock memory and disable otherwise.
	if n.Dev == nil {
		usr, _ := user.Current()
		if usr.Uid != "0" {
			config += "disable_mlock = true\n"
		}
	}

	apiAddr, tls, _, err := n.GetConnectAddr(directory)
	if err != nil {
		return "", fmt.Errorf("failed to infer connection address: %w\n\tuse a non-raft storage backend or add a tcp listener to this cluster", err)
	}

	scheme := "http"
	if tls {
		scheme += "s"
	}

	if n.Storage != nil {
		sConfig, err := n.Storage.ToConfig(directory)
		if err != nil {
			return "", fmt.Errorf("failed to build storage (%#v) to config: %w", n.Storage, err)
		}

		config += sConfig + "\n"

		if _, ok := n.Storage.(*RaftStorage); ok {
			// Need need to compute the cluster address. This is usually
			// one port higher than the api address.
			host, port, err := net.SplitHostPort(apiAddr)
			if err != nil {
				return "", fmt.Errorf("failed to parse API listen address (%v): %w", apiAddr, err)
			}

			clusterPort, err := strconv.Atoi(port)
			if err != nil {
				return "", fmt.Errorf("failed to parse API listen address port (%v): %w", port, err)
			}

			clusterPort += 1
			clusterAddr := fmt.Sprintf("%s:%d", host, clusterPort)

			config += `cluster_addr = "` + scheme + "://" + clusterAddr + `"` + "\n"
		}
	}

	for index, seal := range n.Seals {
		lConfig, err := seal.ToConfig(directory)
		if err != nil {
			return "", fmt.Errorf("failed to build seal %d (%#v) to config: %w", index, seal, err)
		}

		config += lConfig + "\n"
	}

	for index, audit := range n.Audits {
		lConfig, err := audit.ToConfig(directory)
		if err != nil {
			return "", fmt.Errorf("failed to build audit %d (%#v) to config: %w", index, audit, err)
		}

		config += lConfig + "\n"
	}

	if n.Dev == nil {
		config += `api_addr = "` + scheme + "://" + apiAddr + `"` + "\n"

		pluginDir := filepath.Join(directory, "plugins")
		if err := os.MkdirAll(pluginDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create external plugin directory (%v): %w", pluginDir, err)
		}

		config += `plugin_directory = "` + pluginDir + `"` + "\n"

		// Enable sys/raw
		config += "raw_storage_endpoint = true\n"
		config += "introspection_endpoint = true\n"
		config += `log_level = "trace"` + "\n"
	}

	return config, nil
}

func (n *NodeConfig) GetConnectAddr(directory string) (string, bool, string, error) {
	if err := n.Validate(); err != nil {
		return "", false, "", err
	}

	if n.Dev != nil {
		address := n.Dev.Address
		if address == "" {
			address = "127.0.0.1:8200"
		}
		host, err := getConnectionAddr(address)

		var rootCAPath = ""
		if n.Dev.Tls {
			rootCAPath = filepath.Join(directory, "vault-ca.pem")
		}
		return host, n.Dev.Tls, rootCAPath, err
	}

	var lastErr error
	for _, listener := range n.Listeners {
		if tcp, ok := listener.(*TCPListener); ok {
			var addr string
			var ca string
			if addr, ca, lastErr = tcp.GetConnectAddr(directory); lastErr == nil {
				return addr, tcp.TLS != nil, ca, nil
			}
		}
	}

	return "", false, "", fmt.Errorf("unknown connection address for configuration; last error: %w", lastErr)
}

func (n *NodeConfig) AddArgs(directory string) ([]string, error) {
	args := []string{"server", "-exit-on-core-shutdown"}

	if n.Dev != nil {
		devArgs, err := n.Dev.AddArgs(directory)
		if err != nil {
			return nil, fmt.Errorf("failed to build dev mode args to binary: %w", err)
		}

		args = append(args, devArgs...)
	}

	return args, nil
}

type DevConfig struct {
	Token   string `json:"token,omitempty"`
	Address string `json:"address,omitempty"`
	Tls     bool   `json:"tls,omitempty"`
}

func (d *DevConfig) FromInterface(iface map[string]interface{}) error {
	d.Token = iface["token"].(string)
	d.Address = iface["address"].(string)

	if _, present := iface["tls"]; present {
		d.Tls = iface["tls"].(bool)
	}

	return nil
}

func (d *DevConfig) AddArgs(directory string) ([]string, error) {
	args := []string{"-dev"}
	if d.Token != "" {
		args = append(args, fmt.Sprintf("-dev-root-token-id=%s", d.Token))
	}

	if d.Address != "" {
		args = append(args, fmt.Sprintf("-dev-listen-address=%s", d.Address))
	}

	if d.Tls {
		args = append(args, "-dev-tls")
		args = append(args, fmt.Sprintf("-dev-tls-cert-dir=%s", directory))
	}

	return args, nil
}
