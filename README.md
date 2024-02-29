# `devbao`

`devbao` is a CLI utility to start [OpenBao](https://github.com/openbao/openbao)
and HashiCorp Vault instances for **development purposes**.

## Building

To build and run:

```$
make bin
./bin/devbao
```

Because `devbao` is a static Go binary, it should be relocatable to go on `$PATH`.

Data is presently stored in `$HOME/.local/share/devbao`.

Refer to `devbao help` for more information about commands currently
implemented.

## TODO

Below are rough sketches of functionality that could potentially be in
`devbao` in the future.

Feel free to comment on the issue tracker if things are of particular
interest!

 - [ ] Nodes
   - [x] Start node
     - [x] Auto-initialize
     - [x] Apply profile
   - [x] List nodes
   - [x] Stop node
   - [x] Resume node
   - [x] Clean nodes
   - [ ] Transit Seal Config
   - [x] Source environment
   - [x] Access node directory
   - [x] Get/Set unseal keys
   - [x] Get/Set root token (prod)
   - [x] Initialize
   - [x] Seal
   - [X] Unseal
 - [ ] Profiles
   - [x] List profiles
   - [x] Transit Unseal profile
   - [x] PKI profile
   - [ ] Userpass profile
   - [x] Remove profiles
   - [ ] Make profiles configurable
 - [ ] Clusters
   - [ ] Transit Auto-Unseal key cluster + target cluster
   - [ ] OSS HA cluster
    - [ ] Start fresh 3-node cluster
    - [ ] Add HA from existing node
    - [ ] Add node
 - [ ] benchmark-vault integration
 - [ ] Auto-fetch release binaries
 - [ ] Ecosystem integrations
   - [ ] Databases
   - [ ] RabbitMQ
   - [ ] Apache/nginx for certificates
   - [ ] OpenLDAP/389-ds
   - [ ] FreeRADIUS
   - [ ] Run node/cluster on container?
 - [ ] TUI?
