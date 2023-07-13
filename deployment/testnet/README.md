# Prerequisites
Install Docker following the installation guide for Linux OS: [https://docs.docker.com/engine/installation/](https://docs.docker.com/engine/installation/)
* [CentOS](https://docs.docker.com/install/linux/docker-ce/centos) 
* [Ubuntu](https://docs.docker.com/install/linux/docker-ce/ubuntu)

Install docker compose

* [Docker compose](https://docs.docker.com/compose/install/)

## Configuration genesis_testnet.yaml
* Validators 
```
  Validators:
    - Name: Validator-1
      Address: 0x7FdB138136fA26cA833A48f239E42cD334a30F01
      CommissionRate: 100000000000000000
      MaxRate: 250000000000000000
      MaxChangeRate: 50000000000000000
      SelfDelegate: 12500000000000000000000000
      StartWithGenesis: true
    - Name: Validator-2 ....
```

* Accounts
```
    # Genesis Account for distribution
    - Address: 0x14191195F9BB6e54465a341CeC6cce4491599ccC
      Amount: 4140105994018000000000000000
    # Validator
    - Address: 0x7FdB138136fA26cA833A48f239E42cD334a30F01
      Amount: 37500000000000000000000000
    - Address: 0x1E6a2A03CC33924Ef9843217ffc002A1aEBc5342
      Amount: 37500000000000000000000000
    - Address: 0xA0d8447F6E77FE629C2a9EF43e1F7387e442A7ab
      Amount: 37500000000000000000000000
    - Address: 0xbf8e68C0D1363B2F99a4579d325281cC763796ae
      Amount: 37500000000000000000000000
```

## Configuration node_testnet.yaml
* Name, PrivateKey, Seeds
```
Name: <node_name>                             # Example: Validator
PrivateKey: <private_key with non 0x prefix>  # Example: ba92760523019b326e3c73d850nb30da40945a29..............
MainChain:
  Seeds:
    - <public_key_node>@<ip:3000>             # Example: 7fdb138136fa26ca833a48f239e42cd334a30f01@127.0.0.1:3000
    - <public_key_node>@<ip:3000>             # Example: 1e6a2a03cc33924ef9843217ffc002a1aebc5342@127.0.0.2:3000
    - <public_key_node>@<ip:3000>             # Example: a0d8447f6e77fe629c2a9ef43e1f7387e442a7ab@127.0.0.3:3000
    - ...
```

## Starting Testnet
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
docker logs -f --tail 10 testnet-node
