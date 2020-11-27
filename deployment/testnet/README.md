# Prerequisites
Install Docker following the installation guide for Linux OS: [https://docs.docker.com/engine/installation/](https://docs.docker.com/engine/installation/)
* [CentOS](https://docs.docker.com/install/linux/docker-ce/centos) 
* [Ubuntu](https://docs.docker.com/install/linux/docker-ce/ubuntu)

Install docker compose

* [Docker compose](https://docs.docker.com/compose/install/)

## Join Public Testnet
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
