#!/bin/bash

set -e

# Parse command line arguments
SKIP_API_KEY=false

while getopts "Nh" opt; do
    case $opt in
        N)
            SKIP_API_KEY=true
            ;;
        h)
            echo "Usage: $0 [-N] [-h]"
            echo "  -N    Skip updating Datadog API key"
            echo "  -h    Show this help message"
            exit 0
            ;;
        \?)
            echo "Invalid option: -$OPTARG" >&2
            echo "Use -h for help"
            exit 1
            ;;
    esac
done

echo "🚀 Deploying Kubernetes OpenTelemetry Setup with Datadog"

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

# Check prerequisites
print_status "Checking prerequisites..."

if ! command -v docker &> /dev/null; then
    print_error "Docker is required but not installed"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    print_error "kubectl is required but not installed"
    exit 1
fi

# Check if kubectl can connect to cluster
if ! kubectl cluster-info &> /dev/null; then
    print_error "kubectl cannot connect to Kubernetes cluster"
    exit 1
fi

print_status "Prerequisites check passed ✓"

# Build the Docker image
print_status "Building sample application Docker image..."
docker build -t sample-app:latest .

# Detect if we're using minikube or kind and load the image
if kubectl config current-context | grep -q "minikube"; then
    print_status "Detected minikube, loading image..."
    minikube image load sample-app:latest
elif kubectl config current-context | grep -q "kind"; then
    print_status "Detected kind, loading image..."
    kind load docker-image sample-app:latest --name meli-otel-test
else
    print_warning "Unknown cluster type. You may need to push the image to a registry."
fi

# Handle Datadog API Key
if [[ $SKIP_API_KEY == true ]]; then
    print_status "Skipping Datadog API key configuration (using -N flag)"
    
    # Check if the secret exists
    if ! kubectl get secret datadog-api-key -n default &> /dev/null; then
        print_warning "Datadog API key secret does not exist and -N flag was used"
        print_warning "The deployment may fail. Create the secret manually or run without -N flag"
    else
        print_status "Using existing Datadog API key secret"
    fi
else
    print_status "Configuring Datadog API key..."

    # Check if the secret already exists
    if kubectl get secret datadog-api-key -n default &> /dev/null; then
        print_status "Datadog API key secret already exists"
        read -p "Do you want to update it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            kubectl delete secret datadog-api-key -n default
            CREATE_SECRET=true
        else
            CREATE_SECRET=false
        fi
    else
        CREATE_SECRET=true
    fi

    if [[ $CREATE_SECRET == true ]]; then
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
        
        print_status "Datadog API key secret created successfully"
    fi
fi

# Deploy to Kubernetes
print_status "Deploying to Kubernetes..."
kubectl apply -f k8s-manifests.yaml

print_status "Deploying load generator..."
kubectl apply -f load-generator.yaml

# Wait for pods to be ready (with extended timeout for collector startup)
print_status "Waiting for pods to be ready (this may take up to 2 minutes due to collector startup)..."
kubectl wait --for=condition=Ready pod -l app=sample-app --timeout=180s || {
    print_warning "Some pods may still be starting. Checking status..."
    kubectl get pods -l app=sample-app
    echo ""
    print_status "If pods show 1/2 ready, the collector is likely still starting up."
    print_status "Use 'kubectl describe pod <pod-name>' to check events."
}
kubectl wait --for=condition=Ready pod -l app=load-generator --timeout=60s

print_status "Deployment completed successfully! 🎉"

echo ""
print_status "Useful commands:"
echo "  • Check pod status: kubectl get pods"
echo "  • View app logs: kubectl logs -l app=sample-app -c sample-app"
echo "  • View collector logs: kubectl logs -l app=sample-app -c otel-collector"
echo "  • View load generator logs: kubectl logs -l app=load-generator"
echo "  • Port forward app: kubectl port-forward service/sample-app-service 8080:80"
echo "  • Port forward collector metrics: kubectl port-forward service/sample-app-service 8888:8888"
echo ""
print_status "Deployment options:"
echo "  • Deploy without API key prompt: $0 -N"
echo "  • Show help: $0 -h"

echo ""
print_status "Testing endpoints:"
echo "  • Health: curl http://localhost:8080/health"
echo "  • Work: curl http://localhost:8080/work"  
echo "  • Metrics: curl http://localhost:8080/metrics"

echo ""
print_status "Scale commands:"
echo "  • Scale app: kubectl scale deployment sample-app-deployment --replicas=5"
echo "  • Scale load generators: kubectl scale deployment load-generator --replicas=4"

echo ""
print_status "Deployment complete! Check your Datadog account for incoming telemetry data." 