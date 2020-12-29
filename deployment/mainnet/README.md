# Prerequisites
Install Docker following the installation guide for Linux OS: [https://docs.docker.com/engine/installation/](https://docs.docker.com/engine/installation/)
* [CentOS](https://docs.docker.com/install/linux/docker-ce/centos) 
* [Ubuntu](https://docs.docker.com/install/linux/docker-ce/ubuntu)

Install docker compose
* [Docker compose](https://docs.docker.com/compose/install/)
 
## Join Mainnet
```
docker-compose up -d
``` 

### Check if container is running
```
docker-compose ps
```

### Logging
````
docker logs -f --tail 10 kai-mainnet-node
````
for last 10 logs line then follow up