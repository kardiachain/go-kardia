# Deployment on cloud service providers

## Google Container Registry 
Public Docker images are released globally on [GCR](https://cloud.google.com/container-registry/) at [gcr.io/strategic-ivy-130823/go-kardia:milestone3](https://console.cloud.google.com/gcr/images/strategic-ivy-130823/GLOBAL/go-kardia@sha256:9bb6c98dd745d2a85dac3776aae1587dbc75bc5d8b9a19b4031e6935a715362a/details?tab=info&project=strategic-ivy-130823)  
Users can choose this image during their setup, or use below end-2-end scripts.

## Google Cloud Platform deploy testnet
 [GCloud CLI](https://cloud.google.com/sdk/gcloud/) script to deploy Karida testnet on Google cloud.  
  `./gce_deploy_testnet.sh`

   . Starts new GCE virtual machine with required specs & startup script.  
   . Downloads Milestone3 Docker image from GCR.  
   . Starts small testnet of multiple Kardia nodes including Kardia-Eth dual node.  

## Google Cloud Platform join testnet   
  [GCloud CLI](https://cloud.google.com/sdk/gcloud/) script to join Karida testnet on Google cloud.
  `./gce_join_testnet.sh`
   
   . Starts new GCE virtual machine with latest Docker image from GCR.   
   . Join existing public testnet. 
      
## Amazon AWS deploy testnet
[AWS CLI](https://aws.amazon.com/cli/) script to deploy private Kardia testnet on Amazon cloud.  
  `./aws_deploy_testnet.sh`

   . Downloads startup script from Google Cloud Storage.  
   . Create new AWS security group & key pair.  
   . Starts new EC2 virtual machine with required specs & startup script.  
   . Downloads Milestone3 Docker image from GCR.  
   . Starts small testnet of multiple Kardia nodes including Kardia-Eth dual node.  
