$env:GCP_PROJECT = "proyecto1-entrega2-desarrollo"
$env:GCP_REGION = "us-central1"
$env:GCP_ZONE = "us-central1-a"

$env:NETWORK = "mooc-e2-vpc"
$env:SUBNET = "mooc-e2-subnet"
$env:SUBNET_CIDR = "10.20.0.0/24"
$env:PRIVATE_SERVICE_RANGE = "mooc-e2-private-services"
$env:CLOUD_ROUTER = "mooc-e2-router"
$env:CLOUD_NAT = "mooc-e2-nat"

$env:WEB_VM = "mooc-e2-web"
$env:WORKER_VM = "mooc-e2-worker"
$env:WEB_TAG = "web-server"
$env:WORKER_TAG = "worker-server"
$env:WEB_STATIC_IP = "mooc-e2-web-ip"

# Comma-separated public IPs allowed to use SSH directly.
# Leave empty when using IAP.
$env:ADMIN_SOURCE_RANGES = ""

$env:MACHINE_TYPE = "e2-small"
$env:BOOT_DISK_SIZE = "30GB"
$env:IMAGE_FAMILY = "debian-12"
$env:IMAGE_PROJECT = "debian-cloud"
