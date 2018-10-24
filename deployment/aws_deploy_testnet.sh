#!/bin/bash

AMI_ID="ami-0ad99772" # Latest Amazon Linux AMI

echo "Create new security group kardia-sg with custom ports"

SECURITY_ID=`aws ec2 create-security-group --group-name kardia-sg --description "security group for Kardia VM" --query "GroupId"`
aws ec2 authorize-security-group-ingress --group-name kardia-sg --protocol tcp --port 22 --cidr 0.0.0.0/0
aws ec2 authorize-security-group-ingress --group-name kardia-sg  --ip-permissions IpProtocol=tcp,FromPort=3000,ToPort=3010,IpRanges=[{CidrIp=0.0.0.0/0}] IpProtocol=udp,FromPort=3000,ToPort=3010,IpRanges=[{CidrIp=0.0.0.0/0}] IpProtocol=tcp,FromPort=30300,ToPort=30399,IpRanges=[{CidrIp=0.0.0.0/0}] IpProtocol=udp,FromPort=30300,ToPort=30399,IpRanges=[{CidrIp=0.0.0.0/0}] IpProtocol=tcp,FromPort=8545,ToPort=8599,IpRanges=[{CidrIp=0.0.0.0/0}]

echo "Generate new SSH keypair stored at $HOME/kardiavm-key.pem"
aws ec2 create-key-pair --key-name kardiavm-key --query 'KeyMaterial' --output text > kardiavm-key.pem
chmod 400 kardiavm-key.pem

echo "Create new EC2"
EC_ID=`aws ec2 run-instances --image-id $AMI_ID --security-group-ids kardia-sg --count 1 --instance-type t2.medium --block-device-mapping DeviceName=/dev/xvda,Ebs={VolumeSize=30} --key-name kardiavm-key --user-data file://ec2_user_data.txt --query 'Instances[0].InstanceId'`

echo "EC2 $EC_ID is created. Wait for booting up and SSH using key file kardiavm-key.pem"


