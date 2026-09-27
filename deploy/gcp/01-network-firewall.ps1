$ErrorActionPreference = "Stop"

if (-not $env:GCP_PROJECT) {
    throw "Load deploy/gcp/00-variables.example.ps1 first and adjust values."
}

gcloud config set project $env:GCP_PROJECT
gcloud config set compute/region $env:GCP_REGION
gcloud config set compute/zone $env:GCP_ZONE

gcloud compute networks create $env:NETWORK `
    --subnet-mode=custom `
    --bgp-routing-mode=regional

gcloud compute networks subnets create $env:SUBNET `
    --network=$env:NETWORK `
    --region=$env:GCP_REGION `
    --range=$env:SUBNET_CIDR

# Required if Cloud SQL uses private IP / Private Service Access.
gcloud compute addresses create $env:PRIVATE_SERVICE_RANGE `
    --global `
    --purpose=VPC_PEERING `
    --prefix-length=16 `
    --network=$env:NETWORK

gcloud services vpc-peerings connect `
    --service=servicenetworking.googleapis.com `
    --ranges=$env:PRIVATE_SERVICE_RANGE `
    --network=$env:NETWORK

gcloud compute addresses create $env:WEB_STATIC_IP `
    --region=$env:GCP_REGION

gcloud compute firewall-rules create mooc-e2-allow-web `
    --network=$env:NETWORK `
    --allow=tcp:80,tcp:443 `
    --source-ranges=0.0.0.0/0 `
    --target-tags=$env:WEB_TAG `
    --description="Public HTTP/HTTPS only to Web Server"

gcloud compute firewall-rules create mooc-e2-allow-redis-from-web `
    --network=$env:NETWORK `
    --allow=tcp:6379 `
    --source-tags=$env:WEB_TAG `
    --target-tags=$env:WORKER_TAG `
    --description="Redis/asynq reachable only from Web Server"

if ($env:ADMIN_SOURCE_RANGES) {
    gcloud compute firewall-rules create mooc-e2-allow-ssh-admin `
        --network=$env:NETWORK `
        --allow=tcp:22 `
        --source-ranges=$env:ADMIN_SOURCE_RANGES `
        --target-tags=$env:WEB_TAG,$env:WORKER_TAG `
        --description="SSH only from team public IPs"
} else {
    gcloud compute firewall-rules create mooc-e2-allow-iap-ssh `
        --network=$env:NETWORK `
        --allow=tcp:22 `
        --source-ranges=35.235.240.0/20 `
        --target-tags=$env:WEB_TAG,$env:WORKER_TAG `
        --description="SSH through Identity-Aware Proxy"
}

Write-Host "Network, subnet, private service access, static IP and firewall rules created."
