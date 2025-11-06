package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/openbao/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func ClusterInfoFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "type",
			Value: "ha",
			Usage: "type of cluster to run: `ha` for a HA cluster.",
		},
		&cli.StringFlag{
			Name:  "node-type",
			Value: "",
			Usage: "type of node to run: `` for auto-detect preferring OpenBao, `bao` to run an OpenBao instance, or `vault` to run a HashiCorp Vault instance.",
		},
		&cli.IntFlag{
			Name:  "count",
			Value: 3,
			Usage: "number of nodes to run in the cluster; suggested to use an odd number.",
		},
		&cli.IntFlag{
			Name:  "non-voter",
			Value: 0,
			Usage: "number of non-voter nodes to run in the cluster; less than count",
		},
		&cli.StringFlag{
			Name:  "listen",
			Value: "0.0.0.0",
			Usage: "hostname without port to listen on.",
		},
		&cli.IntFlag{
			Name:  "port",
			Value: 8200,
			Usage: "lowest port number to use for clustered nodes; increments by 100.",
		},
		&cli.StringSliceFlag{
			Name:  "seals",
			Value: nil,
			Usage: "URI schemes of seals to add; can be specified multiple times. Use\n\t`http(s)://<TOKEN>@<ADDR>/<MOUNT_PATH>/keys/<KEY_NAME>` for Transit.",
		},
		&cli.StringSliceFlag{
			Name:    "profiles",
			Aliases: []string{"p"},
			Usage:   "profiles to apply to the new node",
		},
	}
}

func BuildClusterStartCommand() *cli.Command {
	c := &cli.Command{
		Name:      "start",
		Aliases:   []string{"s"},
		ArgsUsage: "<cluster-name>",
		Usage:     "start a new cluster and instance nodes",

		Action: RunClusterStartCommand,
	}

	c.Flags = append(c.Flags, ClusterFlags()...)
	c.Flags = append(c.Flags, ClusterInfoFlags()...)

	return c
}

func RunClusterStartCommand(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 1 {
		return fmt.Errorf("missing required positional argument:\n\t<cluster-name>, the name of the cluster to create")
	}

	clusterType := cCtx.String("type")
	if clusterType != "ha" {
		return fmt.Errorf("unknown cluster type: %w\n\tknown options are: `ha`, for a HA cluster", clusterType)
	}

	portBase := cCtx.Int("port")
	if portBase < 1 {
		return fmt.Errorf("minimum bind port is 1; got %v", portBase)
	}

	listen := cCtx.String("listen")

	nonVoter := cCtx.Int("non-voter")
	if nonVoter < 0 {
		return fmt.Errorf("expected at least zero non-voter nodes; got %v", nonVoter)
	}

	count := cCtx.Int("count")
	if count < 1 {
		return fmt.Errorf("required to have at least one node in the cluster; got %v", count)
	} else if nonVoter >= count {
		return fmt.Errorf("required to have at least one voting node in the cluster; have %v non-voters, leaving %v voters", nonVoter, count-nonVoter)
	} else if ((count - nonVoter) % 2) == 0 {
		fmt.Fprintf(os.Stderr, "[warning] it is suggested to have an odd number of voting nodes in the HA cluster")
	}

	clusterName := cCtx.Args().First()
	force := cCtx.Bool("force")
	if !force {
		present, err := bao.ClusterExists(clusterName)
		if err != nil {
			return fmt.Errorf("error checking if cluster exists: %w", err)
		}

		if present {
			return fmt.Errorf("refusing to override cluster %v", clusterName)
		}
	}

	nType := cCtx.String("node-type")

	// Build nodes
	var nodes []*bao.Node
	for index := 0; index < count; index++ {
		nonVoter := index >= (count - nonVoter)

		nvSuffix := ""
		if nonVoter {
			nvSuffix = "-nv"
		}

		name := fmt.Sprintf("%v-node-%d%v", clusterName, index, nvSuffix)
		fmt.Printf("starting %v...\n", name)

		port := portBase + index*100

		var opts []bao.NodeConfigOpt

		opts = append(opts, &bao.RaftStorage{})
		opts = append(opts, &bao.TCPListener{
			Address: fmt.Sprintf("%v:%d", listen, port),
		})

		seals := cCtx.StringSlice("seals")
		for index, seal := range seals {
			url, err := url.Parse(seal)
			if err != nil {
				return fmt.Errorf("failed parsing seal's uri at index %d (`%v`): %w", index, seal, err)
			}

			// Assume transit.

			if url.User == nil || url.User.Username() == "" {
				return fmt.Errorf("malformed or missing user info: expected token in username for Transit: `%v`", url.User.String())
			}

			token := url.User.Username()
			addr := fmt.Sprintf("%v://%v", url.Scheme, url.Host)

			if !strings.Contains(url.Path, "/keys/") {
				return fmt.Errorf("malformed path: no `/keys/` segment: `%v`", url.Path)
			}

			parts := strings.Split(url.Path, "/keys/")
			mount_path := strings.Join(parts[0:len(parts)-1], "/keys")
			key_name := parts[len(parts)-1]

			opts = append(opts, &bao.TransitSeal{
				Address:   addr,
				Token:     token,
				MountPath: mount_path,
				KeyName:   key_name,
			})
		}

		node, err := bao.BuildNode(name, nType, opts...)
		if err != nil {
			return fmt.Errorf("failed to build node %v: %w", name, err)
		}

		node.NonVoter = nonVoter
		if nonVoter {
			if err := node.SaveConfig(); err != nil {
				return fmt.Errorf("failed to save non-voter config for node %v: %w", name, err)
			}
		}

		if err := node.Start(); err != nil {
			return fmt.Errorf("failed to start node %v: %w", name, err)
		}

		if index == 0 {
			// Only initialize the first node; otherwise, additional nodes will
			// not join the cluster.
			if err := node.Initialize(); err != nil {
				return fmt.Errorf("failed to initialize node %v: %w", name, err)
			}

			if _, err := node.Unseal(); err != nil {
				return fmt.Errorf("failed to unseal node %v: %w", name, err)
			}
		}

		nodes = append(nodes, node)
	}

	// Give time for nodes to come up.
	time.Sleep(500 * time.Millisecond)

	// Build initial cluster.
	cluster, err := bao.BuildHACluster(clusterName, nodes[0].Name)
	if err != nil {
		return fmt.Errorf("failed to build cluster: %w", err)
	}

	for _, node := range nodes[1:] {
		// Give time for nodes to join...
		time.Sleep(250 * time.Millisecond)

		fmt.Printf("joining %v to cluster...\n", node.Name)

		if err := cluster.JoinNodeHACluster(node); err != nil {
			return fmt.Errorf("failed to join node %v to cluster: %w", node.Name, err)
		}
	}

	// Give time for cluster to stabilize
	leaderNode, leaderClient, err := cluster.GetLeader()
	errCount := 0
	for err != nil {
		errCount += 1
		if errCount > 5 {
			return fmt.Errorf("failed to find cluster leader: %w", err)
		}

		time.Sleep(time.Duration(errCount) * time.Second)
		leaderNode, leaderClient, err = cluster.GetLeader()
	}

	fmt.Printf("%v selected as leader\n", leaderNode.Name)

	profiles := cCtx.StringSlice("profiles")
	for profileIndex, profile := range profiles {
		warnings, err := bao.ProfileSetup(leaderClient, profile)
		if len(warnings) != 0 || err != nil {
			fmt.Fprintf(os.Stderr, "for profile [%d/%v]:\n", profileIndex, profile)
		}

		for index, warning := range warnings {
			fmt.Fprintf(os.Stderr, " - [warning %d]: %v\n", index, warning)
		}

		if err != nil {
			return err
		}
	}

	return nil
}
