package main

import (
	"fmt"

	"github.com/cipherboy/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func storageFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "storage",
			Value: "raft",
			Usage: "Storage backend to use; choose between `raft`, `file`, or `inmem`. File and Memory backends are not recommended for production use.",
		},
	}
}

func getStorageOpts(cCtx *cli.Context) ([]bao.NodeConfigOpt, error) {
	var opts []bao.NodeConfigOpt

	storage := cCtx.String("storage")
	switch storage {
	case "", "raft":
		opts = append(opts, &bao.RaftStorage{})
	case "file":
		opts = append(opts, &bao.FileStorage{})
	case "inmem":
		opts = append(opts, &bao.InmemStorage{})
	default:
		return nil, fmt.Errorf("unknown value for -storage: `%v`; supported values are `raft`, `file`, or `inmem`", storage)
	}

	return opts, nil
}
