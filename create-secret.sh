#!/bin/bash

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_status "Creating Datadog API key secret..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    print_error "kubectl is required but not installed"
    exit 1
fi

# Check if kubectl can connect to cluster
if ! kubectl cluster-info &> /dev/null; then
    print_error "kubectl cannot connect to Kubernetes cluster"
    exit 1
fi

# Check if secret already exists
if kubectl get secret datadog-api-key -n default &> /dev/null; then
    print_status "Secret already exists. Deleting existing secret..."
    kubectl delete secret datadog-api-key -n default
fi

# Prompt for API key
echo -n "Enter your Datadog API key: "
read -s DD_API_KEY
echo

if [[ -z "$DD_API_KEY" ]]; then
    print_error "Datadog API key is required"
    exit 1
fi

# Create the secret
kubectl create secret generic datadog-api-key \
    --from-literal=api-key="$DD_API_KEY" \
    --namespace=default

print_status "Datadog API key secret created successfully!"
print_status "You can now run './deploy.sh' to deploy the application." 