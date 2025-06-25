#!/bin/bash

set -e

echo "🧹 Cleaning up Kubernetes OpenTelemetry Setup"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Remove Kubernetes resources
print_status "Removing Kubernetes resources..."

if kubectl get -f k8s-manifests.yaml &> /dev/null; then
    kubectl delete -f k8s-manifests.yaml
    print_status "Main application resources removed ✓"
else
    print_warning "Main application resources not found"
fi

if kubectl get -f load-generator.yaml &> /dev/null; then
    kubectl delete -f load-generator.yaml
    print_status "Load generator resources removed ✓"
else
    print_warning "Load generator resources not found"
fi

# Remove Docker image
print_status "Removing Docker image..."
if docker image inspect sample-app:latest &> /dev/null; then
    docker rmi sample-app:latest
    print_status "Docker image removed ✓"
else
    print_warning "Docker image not found"
fi

# Wait for pods to be terminated
print_status "Waiting for pods to terminate..."
kubectl wait --for=delete pod -l app=sample-app --timeout=60s 2>/dev/null || true
kubectl wait --for=delete pod -l app=load-generator --timeout=60s 2>/dev/null || true

print_status "Cleanup completed successfully! 🎉"

echo ""
print_status "Verification commands:"
echo "  • Check pods: kubectl get pods"
echo "  • Check services: kubectl get services"
echo "  • Check configmaps: kubectl get configmaps"
echo "  • Check secrets: kubectl get secrets" 