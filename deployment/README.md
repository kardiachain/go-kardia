# Deployment on cloud service providers

## Google Cloud Platform
Official Go-Kardia Docker image is available for all users on Google Cloud Registry at [gcr.io/strategic-ivy-130823/go-kardia](https://gcr.io/strategic-ivy-130823/go-kardia)  
Users can choose this image when setting up their GCE/Kubernetes nodes, or run below scripts.   

### End-to-end script with GCloud CLI
 [Gcloud](https://cloud.google.com/sdk/gcloud/) script to setup one instance of kardia node:  
    . Creates & starts a new GCE virtual machine with recommended specs & network setup.  
    . Downloads latest Go-Kardia docker from Registry.  
    . Starts the Kardia node.  
  
  
  `./gce_deploy_one_node.sh new-test-node`
  
