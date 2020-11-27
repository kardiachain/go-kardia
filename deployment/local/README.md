# Prerequisites
Install Docker following the installation guide for Linux OS: [https://docs.docker.com/engine/installation/](https://docs.docker.com/engine/installation/)
* [CentOS](https://docs.docker.com/install/linux/docker-ce/centos) 
* [Ubuntu](https://docs.docker.com/install/linux/docker-ce/ubuntu)

Install docker compose

* [Docker compose](https://docs.docker.com/compose/install/)
 
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
