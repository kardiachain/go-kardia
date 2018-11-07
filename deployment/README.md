# Deployment on cloud service providers

## Docker Registry 
Public Docker images are released globally on [GCR](https://cloud.google.com/container-registry/) at [gcr.io/strategic-ivy-130823/go-kardia:v0.4.0](https://gcr.io/strategic-ivy-130823/go-kardia:v0.4.0)  
Users can choose this image during their setup, or use below end-2-end scripts.

## Google Cloud deploy testnet
 [GCloud CLI](https://cloud.google.com/sdk/gcloud/) script to deploy private Karida testnet on multiple virtual machines.  
  `./gce_deploy_testnet.sh`

   . Creates a set of new GCE virtual machines with latest Docker image from GCR.   
   . Starts small testnet of multiple Kardia nodes including subnet of Kardia-Eth dual nodes and Kardia-Neo dual nodes.  
   . Starts KardiaScans UI for network monitoring.

## Google Cloud join whitelisted testnet   
  [GCloud CLI](https://cloud.google.com/sdk/gcloud/) script to join Karida testnet on Google Cloud.  
  `./gce_join_testnet.sh`
   
   . Starts new GCE virtual machine with latest Docker image from GCR.   
   . Joins existing whitelisted testnet. 


## Google Cloud deploy single VM testnet
 [GCloud CLI](https://cloud.google.com/sdk/gcloud/) script to deploy Karida testnet on a single virtual machine.   
  `./gce_deploy_single_machine_testnet.sh`
  
   . Starts new GCE virtual machine with startup script & Docker image from GCR.   
   . Starts small testnet of multiple Kardia nodes including Kardia-Eth dual node.  

## Amazon AWS deploy single VM testnet
[AWS CLI](https://aws.amazon.com/cli/) script to deploy private Kardia testnet on a single virtual machine.   
  `./aws_deploy_single_machine_testnet.sh`

   . Downloads startup script from Google Cloud Storage.  
   . Create new AWS security group & key pair.  
   . Starts new EC2 virtual machine with startup script & Docker image from GCR.   
   . Starts small testnet of multiple Kardia nodes including Kardia-Eth dual node.  
