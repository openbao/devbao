package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cipherboy/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func ServerNameFlags(name string) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "name",
			Value: name,
			Usage: "name for the instance",
		},
	}
}

func ServerFlags(name string) []cli.Flag {
	typeFlags := []cli.Flag{
		&cli.StringFlag{
			Name:  "type",
			Value: "",
			Usage: "type of node to run: `` for auto-detect preferring OpenBao, `bao` to run an OpenBao instance, or `vault` to run a HashiCorp Vault instance.",
		},
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Value:   false,
			Usage:   "overwrite an existing node, if present",
		},
		&cli.StringSliceFlag{
			Name:    "profiles",
			Aliases: []string{"p"},
			Usage:   "profiles to apply to the new node",
		},
		&cli.BoolFlag{
			Name:  "audit",
			Value: false,
			Usage: "enable file auditing of requests",
		},
	}

	return append(ServerNameFlags(name), typeFlags...)
}

func DevServerFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "token",
			Value: "devroot",
			Usage: "development mode root token identifier",
		},
		&cli.StringFlag{
			Name:  "address",
			Value: "127.0.0.1:8200",
			Usage: "development mode listener bind address",
		},
		&cli.BoolFlag{
			Name:  "dev-tls",
			Usage: "enable temporary TLS certificates for this instance",
		},
	}
}

func BuildNodeStartDevCommand() *cli.Command {
	c := &cli.Command{
		Name:    "start-dev",
		Aliases: []string{"d"},
		Usage:   "start a dev-mode instance",

		Action: RunNodeStartDevCommand,
	}

	c.Flags = append(c.Flags, ServerFlags("dev")...)
	c.Flags = append(c.Flags, DevServerFlags()...)

	return c
}

func RunNodeStartDevCommand(cCtx *cli.Context) error {
	if cCtx.Args().Present() {
		return fmt.Errorf("unexpected positional argument -- this command takes none: `%v`", cCtx.Args().First())
	}

	name := cCtx.String("name")
	nType := cCtx.String("type")
	force := cCtx.Bool("force")
	devTls := cCtx.Bool("dev-tls")
	profiles := cCtx.StringSlice("profiles")
	audit := cCtx.Bool("audit")

	if !force {
		present, err := bao.NodeExists(name)
		if err != nil {
			return fmt.Errorf("error checking if node exists: %w", err)
		}

		if present {
			return fmt.Errorf("refusing to overwrite existing node %v", name)
		}
	}
	var opts []bao.NodeConfigOpt
	opts = append(opts, &bao.DevConfig{
		Token:   cCtx.String("token"),
		Address: cCtx.String("address"),
		Tls:     devTls,
	})

	if audit {
		opts = append(opts, &bao.FileAudit{})
		opts = append(opts, &bao.FileAudit{
			CommonAudit: bao.CommonAudit{
				LogRaw: true,
			},
		})
	}

	node, err := bao.BuildNode(name, nType, opts...)
	if err != nil {
		return fmt.Errorf("failed to build node: %w", err)
	}

	if err := node.Start(); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	if err := node.PostInitializeUnseal(); err != nil {
		return fmt.Errorf("failed to apply post-unseal initialization; %w", err)
	}

	client, err := node.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client for node %v: %w", name, err)
	}

	for profileIndex, profile := range profiles {
		warnings, err := bao.ProfileSetup(client, profile)
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

func UnsealFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:    "unseal",
			Aliases: []string{"auto-unseal", "u"},
			Value:   false,
			Usage:   "Automatically unseal the underlying node; requires --initialize",
		},
	}
}

func ProdServerFlags() []cli.Flag {
	ret := []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "listeners",
			Value: cli.NewStringSlice("tcp:0.0.0.0:8200"),
			Usage: "Bind address of the listener to add; can be specified multiple times. Use\n\t`tcp:` to prefix network listener bind addresses, or\n\t`unix:` to prefix socket listener paths.",
		},
		&cli.StringFlag{
			Name:  "storage",
			Value: "raft",
			Usage: "Storage backend to use; choose between `raft`, `postgresql`, `file`, or `inmem`. File and Memory backends are not recommended for production use.",
		},
		&cli.BoolFlag{
			Name:    "initialize",
			Aliases: []string{"auto-initialize", "i"},
			Value:   false,
			Usage:   "Automatically initialize the underlying node, saving unseal keys",
		},
		&cli.StringSliceFlag{
			Name:  "seals",
			Value: nil,
			Usage: "URI schemes of seals to add; can be specified multiple times. Use\n\t`http(s)://<TOKEN>@<ADDR>/<MOUNT_PATH>/keys/<KEY_NAME>` for Transit.",
		},
	}

	ret = append(ret, UnsealFlags()...)
	return ret
}

func BuildNodeStartCommand() *cli.Command {
	c := &cli.Command{
		Name:    "start",
		Aliases: []string{"s"},
		Usage:   "start a production instance",

		Action: RunNodeStartCommand,
	}

	c.Flags = append(c.Flags, ServerFlags("prod")...)
	c.Flags = append(c.Flags, ProdServerFlags()...)

	return c
}

func RunNodeStartCommand(cCtx *cli.Context) error {
	if cCtx.Args().Present() {
		return fmt.Errorf("unexpected positional argument -- this command takes none: `%v`", cCtx.Args().First())
	}

	name := cCtx.String("name")
	nType := cCtx.String("type")
	storage := cCtx.String("storage")
	initialize := cCtx.Bool("initialize")
	unseal := cCtx.Bool("unseal")
	force := cCtx.Bool("force")
	profiles := cCtx.StringSlice("profiles")
	audit := cCtx.Bool("audit")

	if !force {
		present, err := bao.NodeExists(name)
		if err != nil {
			return fmt.Errorf("error checking if node exists: %w", err)
		}

		if present {
			return fmt.Errorf("refusing to overwrite existing node %v", name)
		}
	}

	if unseal && !initialize {
		return fmt.Errorf("--unseal requires --initialize, but was not provided")
	}

	if len(profiles) > 0 && !unseal {
		return fmt.Errorf("using --profiles requires --unseal and --initialize")
	}

	var opts []bao.NodeConfigOpt

	switch storage {
	case "", "raft":
		opts = append(opts, &bao.RaftStorage{})
	case "postgresql":
		opts = append(opts, &bao.PostgresqlStorage{})
	case "file":
		opts = append(opts, &bao.FileStorage{})
	case "inmem":
		opts = append(opts, &bao.InmemStorage{})
	default:
		return fmt.Errorf("unknown value for -storage: `%v`; supported values are `raft`, `postgresql`, `file`, or `inmem`", storage)
	}

	listeners := cCtx.StringSlice("listeners")
	for index, listener := range listeners {
		if strings.HasPrefix(listener, "tcp:") {
			opts = append(opts, &bao.TCPListener{
				Address: strings.TrimPrefix(listener, "tcp:"),
			})
		} else if strings.HasPrefix(listener, "unix:") {
			opts = append(opts, &bao.UnixListener{
				Path: strings.TrimPrefix(listener, "unix:"),
			})
		} else {
			return fmt.Errorf("unknown type prefix for -listeners at index %d: `%v`; supported values are `tcp:<bind address>` or `unix:<path>`", index, listener)
		}
	}

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

	if audit {
		opts = append(opts, &bao.FileAudit{})
		opts = append(opts, &bao.FileAudit{
			CommonAudit: bao.CommonAudit{
				LogRaw: true,
			},
		})
	}

	node, err := bao.BuildNode(name, nType, opts...)
	if err != nil {
		return fmt.Errorf("failed to build node: %w", err)
	}

	if err := node.Start(); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	if initialize {
		if err := node.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize node: %w", err)
		}

		if unseal {
			if _, err := node.Unseal(); err != nil {
				return fmt.Errorf("failed to unseal node: %w", err)
			}

			// TODO: use a client request with proper back-off to determine
			// when the node is responding.
			time.Sleep(500 * time.Millisecond)

			if err := node.PostInitializeUnseal(); err != nil {
				return fmt.Errorf("failed to apply post-unseal initialization; %w", err)
			}
		}
	}

	client, err := node.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get client for node %v: %w", name, err)
	}

	for profileIndex, profile := range profiles {
		warnings, err := bao.ProfileSetup(client, profile)
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
