package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/cipherboy/devbao/pkg/bao"

	"github.com/urfave/cli/v2"
)

func sealFlags() []cli.Flag {
	ret := []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "seals",
			Value: nil,
			Usage: "URI schemes of seals to add; can be specified multiple times. Use\n\t`http(s)://<TOKEN>@<ADDR>/<MOUNT_PATH>/keys/<KEY_NAME>` for Transit,\n\t`static://<key, optional>` for static.",
		},
	}

	return ret
}

func getSealsOpts(cCtx *cli.Context) ([]bao.NodeConfigOpt, error) {
	var opts []bao.NodeConfigOpt

	seals := cCtx.StringSlice("seals")
	for index, seal := range seals {
		url, err := url.Parse(seal)
		if err != nil {
			return nil, fmt.Errorf("failed parsing seal's uri at index %d (`%v`): %w", index, seal, err)
		}

		switch url.Scheme {
		case "http", "https":
			// Assume transit.
			if url.User == nil || url.User.Username() == "" {
				return nil, fmt.Errorf("malformed or missing user info: expected token in username for Transit: `%v`", url.User.String())
			}

			token := url.User.Username()
			addr := fmt.Sprintf("%v://%v", url.Scheme, url.Host)

			if !strings.Contains(url.Path, "/keys/") {
				return nil, fmt.Errorf("malformed path: no `/keys/` segment: `%v`", url.Path)
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
		case "static":
			seal := &bao.StaticSeal{}
			if url.Host != "" {
				seal.CurrentKey = url.Host
			} else {
				data := make([]byte, 32)
				if _, err := io.ReadFull(rand.Reader, data); err != nil {
					return nil, fmt.Errorf("failed to generate random key for static seal: %w", err)
				}
				seal.CurrentKey = hex.EncodeToString(data)
			}
			opts = append(opts, seal)
		default:
			return nil, fmt.Errorf("unknown type of URL: %v", url.Scheme)
		}
	}

	return opts, nil
}
