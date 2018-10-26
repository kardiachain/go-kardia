#!/bin/bash

IMAGE_NAME=gcr.io/strategic-ivy-130823/go-kardia:milestone3

gcloud compute instances create kardia-testnet \
--machine-type=n1-standard-1 \
--subnet=default \
--network-tier=PREMIUM \
--metadata=google-logging-enabled=true,startup-script-url=https://storage.googleapis.com/kardia-startup-scripts/gce_startup.sh \
--maintenance-policy=MIGRATE \
--tags=http-server,https-server \
--image=cos-stable-70-11021-51-0 \
--image-project=cos-cloud \
--boot-disk-size=30GB \
--boot-disk-type=pd-standard \
--boot-disk-device-name=kardia-testnet \
