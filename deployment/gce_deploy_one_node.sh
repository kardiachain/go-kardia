#!/bin/bash

if [ "$#" -ne 1 ]; then
  echo "Missing node name argument"
  exit 1
fi

gcloud beta compute instances create-with-container $1 \
--machine-type=n1-standard-1 \
--subnet=default \
--network-tier=PREMIUM \
--metadata=google-logging-enabled=true \
--maintenance-policy=MIGRATE \
--tags=http-server,https-server \
--image=cos-stable-70-11021-51-0 \
--image-project=cos-cloud \
--boot-disk-size=30GB \
--boot-disk-type=pd-standard \
--boot-disk-device-name=$1 \
--container-image=gcr.io/strategic-ivy-130823/go-kardia:latest \
--container-restart-policy=on-failure \
--container-privileged \
--container-arg="-name=$1" \
--container-arg="-dev" --container-arg="-mainChainValIndex=1" \
--container-arg="-rpc" --container-arg="-rpcport=8545"

