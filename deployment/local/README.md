- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Setup](#setup)
  - [Configuration](#configuration)
    - [Genesis file](#genesis-file)
    - [Network type](#network-type)
    - [Node config](#node-config)
  - [Build image from source](#build-image-from-source)
  - [Create empty folders for storing node data](#create-empty-folders-for-storing-node-data)
  - [Run Local network](#run-local-network)
    - [Container running](#container-running)
    - [Logging](#logging)


# Introduction
This section tries to set up a network of 3 nodes.

# Prerequisites
- Docker and Docker Compose

# Setup

## Configuration

### Genesis file
Replace or modify the content of [`./genesis.yaml`](./genesis.yaml) as you desire.

### Network type
In `docker-compose.yaml` file, the network type is preset with `testnet`.

### Node config
In `docker-compose.yaml` file, config files `node1.yaml`, `node2.yaml` and `node3.yaml` are preset to corresponding nodes. 
In each file, the fields are self-explained, for examples:
```yaml
# specifies seed mode for a node
- SeedMode: true

# specifies persistent peers
- PersistentPeers:
    - 7cefc13b6e2aedeedfb7cb6c32457240746baee5@node2:3000
    - ff3dac4f04ddbd24de5d6039f90596f0a8bb08fd@node3:3000  

# to skip special addresses like localhost, 0.0.0.0,... etc
- AddrBookStrict: false

# node's log level: crit, error, warn, info, debug, trace
- LogLevel: info
```

## Build image from source
```
# wd: go-kardia/deployment/local
# build a new image from source code with tag `go-kardia:latest`
docker-compose build 
```

## Create empty folders for storing node data
```
# wd: go-kardia/deployment/local
mkdir node1 node2 node3
```


## Run Local network 
```
docker-compose up -d
``` 

### Container running

```
docker-compose ps
```

### Logging
````
docker logs -f --tail 10 node1
docker logs -f --tail 10 node2
docker logs -f --tail 10 node3
