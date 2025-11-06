# `devbao`

`devbao` is a CLI utility to start [OpenBao](https://github.com/openbao/openbao)
and HashiCorp Vault instances for **development purposes**.

This allows you to skip many common steps such as creating configuration,
initializing the instance, managing root tokens through the use of CLI flags
and a common configuration directory.

Missing an option? Open a pull request!

## Install

```$
$ go install github.com/openbao/devbao/cmd/devbao@main
```

## Building

To build and run:

```$
$ make bin
$ ./bin/devbao
```

Because `devbao` is a static Go binary, it should be relocatable anywhere on
`$PATH`.

Data is presently stored in `$HOME/.local/share/devbao`.

## CLI interface

Refer to `devbao help` for more information about commands currently
implemented.

With Bash, a node could be created and connected with:

```$
# This starts a production (persistent) single node, initializing (to save the
# root token and unseal keys), unsealing (to make it usable), and provisioning
# a root and intermediate PKI mount (the `pki` profile).
$ devbao node start --force --unseal --initialize --profiles pki

# This loads the environment details to contact this instance into the shell
# session so that future `bao` commands will work.
$ . <(devbao node env prod)

$ bao secrets list
```

HA cluster can similarly be created with the `devbao cluster start <name>`
command.

## TUI interface

`devbao` features a basic TUI available under the `devbao tui` command.

## Contributing

Interested in contributing? Consider opening an issue to discuss the feature
before opening a PR.

See the [contributing guidelines](https://github.com/openbao/openbao/tree/main/CONTRIBUTING.md)
in the OpenBao project as they apply here as well.
