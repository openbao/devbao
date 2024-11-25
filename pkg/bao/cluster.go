package bao

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/openbao/openbao/api/v2"
)

const (
	HAClusterType string = "HA"
)

const (
	ClusterJsonName = "cluster.json"
)

type Cluster struct {
	Name string `json:"name"`
	Type string `json:"type"`

	Nodes []string `json:"nodes"`
}

func BuildHACluster(clusterName string, nodeName string) (*Cluster, error) {
	c := &Cluster{
		Name: clusterName,
		Type: HAClusterType,
	}

	node, err := LoadNode(nodeName)
	if err != nil {
		return nil, fmt.Errorf("error loading node to add to cluster: %w", err)
	}

	if node.Cluster != "" {
		return nil, fmt.Errorf("node `%v` already present in different cluster: `%v`", nodeName, node.Cluster)
	}

	if node.Config.Dev != nil {
		return nil, fmt.Errorf("node `%v` is a dev-node cluster; ephemeral nodes cannot be added to clusters", nodeName)
	}

	node.Cluster = clusterName
	if err := node.SaveConfig(); err != nil {
		return nil, fmt.Errorf("failed saving node `%v` while adding to cluster: %w", nodeName, err)
	}

	c.Nodes = append(c.Nodes, nodeName)

	if err := c.SaveConfig(); err != nil {
		return nil, fmt.Errorf("error saving cluster configuration: %w", err)
	}

	return c, nil
}

func ListClusters() ([]string, error) {
	dir := ClusterBaseDirectory()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cluster directory (%v): %w", dir, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("error listing cluster directory (`%v`): %w", dir, err)
	}

	var results []string
	for _, entry := range entries {
		if entry.IsDir() {
			results = append(results, entry.Name())
		}
	}

	return results, nil
}

func ClusterExists(name string) (bool, error) {
	clusters, err := ListClusters()
	if err != nil {
		return false, err
	}

	for _, cluster := range clusters {
		if cluster == name {
			return true, nil
		}
	}

	return false, nil
}

func LoadClusterUnvalidated(name string) (*Cluster, error) {
	var cluster Cluster
	cluster.Name = name

	if err := cluster.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to read cluster (%v) configuration: %w", name, err)
	}

	return &cluster, nil
}

func LoadCluster(name string) (*Cluster, error) {
	cluster, err := LoadClusterUnvalidated(name)
	if err != nil {
		return nil, err
	}

	if err := cluster.Validate(); err != nil {
		return nil, fmt.Errorf("invalid cluster (%v) configuration: %w", name, err)
	}

	return cluster, nil
}

func (c *Cluster) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("missing cluster name")
	}

	if c.Type != HAClusterType {
		return fmt.Errorf("unknown cluster type: %v", c.Type)
	}

	for index, name := range c.Nodes {
		node, err := LoadNode(name)
		if err != nil {
			return fmt.Errorf("failed loading node (%d / %v): %w", index, name, err)
		}

		if node.Cluster != c.Name {
			return fmt.Errorf("node (%d / %v) not listed in cluster; listed in `%v`", index, name, node.Cluster)
		}
	}

	return nil
}

func (c *Cluster) LoadConfig() error {
	directory := c.GetDirectory()
	path := filepath.Join(directory, ClusterJsonName)
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

	if err := c.FromInterface(cfg); err != nil {
		return fmt.Errorf("failed to translate config: %w", err)
	}

	return nil
}

func (c *Cluster) FromInterface(iface map[string]interface{}) error {
	c.Name = iface["name"].(string)
	c.Type = iface["type"].(string)

	if nodesRaw, ok := iface["nodes"].([]interface{}); ok {
		c.Nodes = nil
		for _, nodeRaw := range nodesRaw {
			c.Nodes = append(c.Nodes, nodeRaw.(string))
		}
	}

	return nil
}

func ClusterBaseDirectory() string {
	usr, _ := user.Current()
	dir := usr.HomeDir

	return filepath.Join(dir, ".local/share/devbao/clusters")
}

func (c *Cluster) GetDirectory() string {
	return filepath.Join(ClusterBaseDirectory(), c.Name)
}

func (c *Cluster) SaveConfig() error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("failed validating config prior to saving: %w", err)
	}

	directory := c.GetDirectory()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("failed to create cluster directory (%v): %w", directory, err)
	}

	path := filepath.Join(directory, ClusterJsonName)
	configFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open config file (`%v`) for writing: %w", path, err)
	}

	defer configFile.Close()

	if err := json.NewEncoder(configFile).Encode(c); err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return nil
}

func (c *Cluster) GetLeader() (*Node, *api.Client, error) {
	var errors *multierror.Error
	for index, name := range c.Nodes {
		node, err := LoadNode(name)
		if err != nil {
			err = fmt.Errorf("error loading node %d / %v: %w", index, name, err)
			errors = multierror.Append(errors, err)
			continue
		}

		client, err := node.GetClient()
		if err != nil {
			err = fmt.Errorf("error getting client for node %d / %v: %w", index, name, err)
			errors = multierror.Append(errors, err)
			continue
		}

		resp, err := client.Sys().Leader()
		if err != nil {
			err = fmt.Errorf("error getting leadership status for node %d / %v: %w", index, name, err)
			errors = multierror.Append(errors, err)
			continue
		}

		if resp.IsSelf {
			return node, client, nil
		}
	}

	err := fmt.Errorf("no leader found on cluster; %v nodes; got the following errors: %w", len(c.Nodes), errors)
	return nil, nil, err
}

func (c *Cluster) JoinNodeHACluster(node *Node) error {
	leaderNode, leaderClient, err := c.GetLeader()
	if err != nil {
		return fmt.Errorf("error finding leader: %w", err)
	}

	nodeClient, err := node.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client for node to add: %w", err)
	}

	if len(leaderNode.Config.Seals) != len(node.Config.Seals) {
		return fmt.Errorf("mismatched seal configuration counts between %v and %v; cannot join existing cluster -- ensure seals are configured correctly and retry", leaderNode.Name, node.Name)
	}

	for index, seal := range leaderNode.Config.Seals {
		leaderConfig, err := seal.ToConfig("/")
		if err != nil {
			return fmt.Errorf("error building seal config %d for %v: %w", index, leaderNode.Name, err)
		}

		followerConfig, err := node.Config.Seals[index].ToConfig("/")
		if err != nil {
			return fmt.Errorf("error building seal config %d for %v: %w", index, node.Name, err)
		}

		if leaderConfig != followerConfig {
			return fmt.Errorf("mismatched seal configuration counts between %v and %v; cannot join existing cluster -- ensure seals are configured correctly and retry", leaderNode.Name, node.Name)
		}
	}

	resp, err := nodeClient.Sys().RaftJoin(&api.RaftJoinRequest{
		LeaderAPIAddr: leaderClient.Address(),
		Retry:         true,
		NonVoter:      node.NonVoter,
	})
	if err != nil {
		return fmt.Errorf("failed joining node %v to cluster %v / leader %v: %w", node.Name, c.Name, leaderNode.Name, err)
	}

	// Update this node's token to mirror the leadership.
	node.Token = leaderNode.Token
	node.UnsealKeys = leaderNode.UnsealKeys
	node.Cluster = c.Name

	if err := node.SaveConfig(); err != nil {
		return fmt.Errorf("failed saving updated state for joined node %v: %w", node.Name, err)
	}

	if !resp.Joined {
		time.Sleep(500 * time.Millisecond)

		// Attempt to unseal using stored shamir's keys.
		if _, err := node.Unseal(); err != nil {
			return fmt.Errorf("failed unsealing follower node %v: %w", node.Name, err)
		}

		time.Sleep(500 * time.Millisecond)
	}

	c.Nodes = append(c.Nodes, node.Name)
	if err := c.SaveConfig(); err != nil {
		return fmt.Errorf("failed saving updated cluster state: %w", err)
	}

	return nil
}

func (c *Cluster) RemoveNodeHACluster(node *Node) error {
	_, leaderClient, err := c.GetLeader()
	if err != nil {
		return fmt.Errorf("error finding leader: %w", err)
	}

	nodeAddr, _, err := node.GetConnectAddr()
	if err != nil {
		return fmt.Errorf("failed to get node's address: %w", err)
	}

	// Inferring the node_id from the API address is difficult; we need to
	// fetch the ha-status to find the API address->cluster address mappings
	// and then find the server with the given cluster address.
	statusResp, err := leaderClient.Logical().Read("sys/ha-status")
	if err != nil {
		return fmt.Errorf("error reading raft configuration from node %v: %w", node.Name, err)
	}

	clusterAddr := ""
	nodes := statusResp.Data["nodes"].([]interface{})
	for _, nodeRaw := range nodes {
		node := nodeRaw.(map[string]interface{})
		apiAddr := node["api_address"].(string)
		if apiAddr == nodeAddr {
			clusterAddr = node["cluster_address"].(string)
			break
		}
	}

	if clusterAddr == "" {
		// This node might've manually been removed from the cluster or never
		// really joined to it.
		nodeIndex := -1
		for index, name := range c.Nodes {
			if name == node.Name {
				nodeIndex = index
				break
			}
		}

		if nodeIndex != -1 {
			nodesBefore := c.Nodes[0:nodeIndex]
			nodesAfter := c.Nodes[nodeIndex+1:]
			c.Nodes = append(nodesBefore, nodesAfter...)

			if err := c.SaveConfig(); err != nil {
				return fmt.Errorf("failed to save cluster %v after node removal: %w", c.Name, err)
			}
		}

		return nil

	}

	cfgResp, err := leaderClient.Logical().Read("sys/storage/raft/configuration")
	if err != nil {
		return fmt.Errorf("error reading raft configuration from node %v: %w", node.Name, err)
	}

	raftId := ""
	cfg := cfgResp.Data["config"].(map[string]interface{})
	servers := cfg["servers"].([]interface{})
	for _, serverRaw := range servers {
		server := serverRaw.(map[string]interface{})
		addr := server["address"].(string)
		if strings.Contains(clusterAddr, addr) {
			raftId = server["node_id"].(string)
			break
		}
	}

	if raftId == "" {
		return fmt.Errorf("could not find node %v's raft ID based on sys/storage/raft/configuration response", node.Name)
	}

	_, err = leaderClient.Logical().Write("sys/storage/raft/remove-peer", map[string]interface{}{
		"server_id": raftId,
	})
	if err != nil {
		return fmt.Errorf("failed removing node %v from cluster: %w", node.Name, err)
	}

	node.Cluster = ""
	if err := node.SaveConfig(); err != nil {
		return fmt.Errorf("failed saving updated state for joined node %v: %w", node.Name, err)
	}

	nodeIndex := -1
	for index, name := range c.Nodes {
		if name == node.Name {
			nodeIndex = index
			break
		}
	}

	if nodeIndex != -1 {
		nodesBefore := c.Nodes[0:nodeIndex]
		nodesAfter := c.Nodes[nodeIndex+1:]
		c.Nodes = append(nodesBefore, nodesAfter...)

		if err := c.SaveConfig(); err != nil {
			return fmt.Errorf("failed to save cluster %v after node removal: %w", c.Name, err)
		}
	}

	return nil
}

func (c *Cluster) Clean(force bool) error {
	for index, name := range c.Nodes {
		node, err := LoadNode(name)
		if err != nil {
			if strings.Contains(err.Error(), "no such file or directory") {
				continue
			}

			// Some other unknown error.
			if !force {
				return fmt.Errorf("failed to load node %d / %v to determine state: %w", index, name, err)
			} else {
				node = &Node{
					Name: name,
				}
			}
		}

		if node.Exec != nil {
			if err := node.Exec.ValidateRunning(); err == nil {
				err := node.Kill()
				if err != nil {
					if !force {
						return fmt.Errorf("failed to stop node prior to removal: %w", err)
					}
				}
			}
		}

		err = node.Clean(force)
		if err != nil {
			if !force {
				return fmt.Errorf("failed to clean node %d / %v: %w", index, name, err)
			}
		}
	}

	directory := c.GetDirectory()
	return os.RemoveAll(directory)
}
