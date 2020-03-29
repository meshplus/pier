# Pier

## Build

Using the follow command to install necessary tools.

```bash
make init
```

And then install bitxmesh using the follow command.

```bash
make install
```

## Configuration

```toml
title = "pier"

[port]
pprof = 44556

[log]
level = "debug"

[bitxhub]
addr = "localhost:60011"

[appchain]
type = "fabric" 
[appchain.fabric]
  "addr" = "139.219.105.188:10053"
```

`port.pprof`: the pprof server port

`log.level`: log level: debug, info, warn, error, fatal

`bitxhub.addr`: bitxhub grpc server port

`appchain.type`: fabric only

`appchain.validators`: appchain validators address

`appchain.fabric.addr`: fabric address

## Usage

Using the follow command to initialize pier.
```bash
pier init
```
Default repo path is `~/.pier`. If you want to specify the repo path, you can use `--repo` flag.

```bash
pier init --repo=/Users/xcc/.pier_fabric
```

After initializing pier, it will generate the follow directory:

```
~/.pier
├── pier.account
├── pier.toml
├── fabric
│   ├── config_fab.yaml
│   └── crypto-config
└── plugins
    ├── fabric-client-1.0.so
    └── hpc-client.so
```

Depending on the type of appchain, you should write your config in the `pier.toml`.
                                              
Next, launch your pier:

```bash
pier start
```