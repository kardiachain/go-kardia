- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Setup](#setup)
  - [Config](#config)
    - [Genesis file \& network type](#genesis-file--network-type)
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

## Config
### Genesis file & network type
TODO

### Node config
TODO
- SeedMode
  - true
- PersistentPeers:
    - 7cefc13b6e2aedeedfb7cb6c32457240746baee5@node2:3000
    - ff3dac4f04ddbd24de5d6039f90596f0a8bb08fd@node3:3000  
- AddrBookStrict:
  - false: to skip localhost, 0.0.0.0,... etc
- LogLevel
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
docker-compose build
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
