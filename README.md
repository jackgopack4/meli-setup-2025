# Kubernetes OpenTelemetry Setup with Datadog

This project demonstrates a complete Kubernetes deployment of a sample application that generates OpenTelemetry telemetry data (traces, metrics, and logs) with an OpenTelemetry Collector sidecar that forwards data to Datadog using both the Datadog exporter and connector.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Kubernetes Pod                        │
│  ┌─────────────────┐              ┌─────────────────────┐  │
│  │   Sample App    │              │  OTel Collector    │  │
│  │                 │              │                     │  │
│  │ - Generates     │──── OTLP ────│ Receivers:          │  │
│  │   Traces        │    HTTP      │ - OTLP              │  │
│  │ - Generates     │    :4318     │ - Prometheus        │  │
│  │   Metrics       │              │                     │  │
│  │ - Generates     │              │ Processors:         │  │
│  │   Logs          │              │ - Resource          │  │
│  │                 │              │ - Batch             │  │
│  │ Port: 8080      │              │ - Memory Limiter    │  │
│  └─────────────────┘              │                     │  │
│                                   │ Connectors:         │  │
│                                   │ - Datadog           │  │
│                                   │                     │  │
│                                   │ Exporters:          │  │
│                                   │ - Datadog/traces    │  │
│                                   │ - Datadog/metrics   │  │
│                                   │ - OTLP/datadog      │  │
│                                   └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
                                    ┌─────────────┐
                                    │   Datadog   │
                                    │   Platform  │
                                    └─────────────┘
```

## Components

### Sample Application
- **Language**: Go
- **Framework**: Standard HTTP server with OpenTelemetry instrumentation
- **Telemetry**: Generates traces, metrics, and logs
- **Endpoints**:
  - `/health` - Health check endpoint
  - `/work` - Simulates work with nested spans and random errors
  - `/metrics` - Returns system metrics

### OpenTelemetry Collector
- **Deployment**: Sidecar container alongside the sample app
- **Receivers**: OTLP (gRPC and HTTP), Prometheus
- **Processors**: Resource, Batch, Memory Limiter
- **Connectors**: Datadog connector for span-to-metrics conversion
- **Exporters**: Datadog exporter for traces and metrics, OTLP for logs

### Load Generator
- **Purpose**: Automatically generates traffic to produce telemetry data
- **Deployment**: Separate pods that continuously call the sample app endpoints

## Prerequisites

1. **Kubernetes cluster** (local or cloud)
2. **Docker** for building the sample application
3. **Datadog account** with API key
4. **kubectl** configured to access your cluster

## Security Features

This setup implements secure API key management:

- **No hardcoded secrets**: The Datadog API key is never stored in the manifest files
- **Runtime secret creation**: API keys are prompted for during deployment or created manually
- **Kubernetes secrets**: API keys are stored securely using Kubernetes native secrets
- **Helper scripts**: Convenient scripts (`deploy.sh`, `create-secret.sh`) handle secret creation
- **Reusable secrets**: The deployment script checks for existing secrets and offers to reuse them

## Setup Instructions

### 1. Build the Sample Application

```bash
# Build the Docker image
docker build -t sample-app:latest .

# If using minikube or kind, load the image
# For minikube:
minikube image load sample-app:latest

# For kind:
kind load docker-image sample-app:latest
```

### 2. Configure Datadog API Key

**⚠️ Security Notice**: The API key is no longer hardcoded in the manifests for security reasons.

Choose one of the following methods:

#### Option A: Automatic deployment with prompted API key (Recommended)
```bash
# Run the deployment script - it will prompt for your API key
./deploy.sh
```

#### Option B: Create the secret manually first
```bash
# Create the secret manually using the helper script
./create-secret.sh

# Then deploy the manifests
kubectl apply -f k8s-manifests.yaml
kubectl apply -f load-generator.yaml
```

#### Option C: Create secret using kubectl directly
```bash
# Create the secret directly with kubectl
kubectl create secret generic datadog-api-key \
    --from-literal=api-key="YOUR_ACTUAL_API_KEY" \
    --namespace=default

# Then deploy the manifests
kubectl apply -f k8s-manifests.yaml
kubectl apply -f load-generator.yaml
```

### 3. Verify the Deployment

```bash
# Check pod status
kubectl get pods -l app=sample-app

# Check logs
kubectl logs -l app=sample-app -c sample-app
kubectl logs -l app=sample-app -c otel-collector

# Port forward to access the application locally
kubectl port-forward service/sample-app-service 8080:80

# Test the endpoints
curl http://localhost:8080/health
curl http://localhost:8080/work
curl http://localhost:8080/metrics
```

### 4. Monitor Traffic Generation

```bash
# Check load generator logs
kubectl logs -l app=load-generator

# Monitor the application receiving requests
kubectl logs -l app=sample-app -c sample-app -f
```

## Configuration Details

### OpenTelemetry Collector Configuration

The collector is configured with:

- **OTLP Receivers**: Accept telemetry data from the sample app
- **Prometheus Receiver**: Scrape metrics from the app's metrics endpoint
- **Datadog Connector**: Converts spans to APM metrics and adds Datadog-specific enrichment
- **Datadog Exporters**: Send data directly to Datadog with proper formatting
- **Resource Processor**: Adds consistent resource attributes
- **Batch Processor**: Optimizes data transmission

### Sample Application Features

- **Distributed Tracing**: Creates spans with parent-child relationships
- **Custom Metrics**: Tracks request counts and duration histograms
- **Error Simulation**: Randomly generates errors for realistic telemetry
- **Resource Attributes**: Includes service name, version, and environment

## Expected Datadog Data

After deployment, you should see in Datadog:

### APM (Traces)
- Service: `sample-app`
- Operations: `health_check`, `do_work`, `nested_operation`, `metrics`
- Error traces when the app simulates failures

### Metrics
- `http_requests_total` - Counter of HTTP requests by endpoint and status
- `http_request_duration_seconds` - Histogram of request durations
- APM metrics generated by the Datadog connector

### Infrastructure
- Container and Kubernetes metrics from the Datadog agent (if deployed)

## Scaling and Customization

### Increase Load
```bash
# Scale up the sample app
kubectl scale deployment sample-app-deployment --replicas=5

# Scale up the load generators
kubectl scale deployment load-generator --replicas=4
```

### Modify Telemetry
- Edit the Go application in `main.go` to add more instrumentation
- Modify the collector configuration in the ConfigMap
- Adjust the load generator script for different traffic patterns

## Troubleshooting

### Collector Pod Restarts
If you see collector pods restarting frequently, this is usually due to slow startup:

```bash
# Check pod status and restart counts
kubectl get pods -l app=sample-app

# Look for specific restart reasons
kubectl describe pod <pod-name>

# Check collector logs for startup issues
kubectl logs <pod-name> -c otel-collector --previous
```

**Common Solutions:**
- The manifests now include a startup probe with 60s timeout
- Liveness probes are configured with generous timeouts
- If still failing, check your internet connection to Datadog

### Check Collector Health
```bash
# Check if health endpoint is responding
kubectl port-forward service/sample-app-service 13133:13133
curl http://localhost:13133/

# Access collector metrics endpoint  
kubectl port-forward service/sample-app-service 8888:8888
curl http://localhost:8888/metrics
```

### Debug Telemetry Flow
```bash
# Enable debug logging in the collector
# Edit the ConfigMap and set telemetry.logs.level to "debug"
kubectl edit configmap otel-collector-config

# Restart the deployment
kubectl rollout restart deployment sample-app-deployment
```

### Verify Datadog Connection
```bash
# Check collector logs for Datadog export success/failures
kubectl logs -l app=sample-app -c otel-collector | grep -i datadog

# Look for authentication or network errors
kubectl logs -l app=sample-app -c otel-collector | grep -i error
```

### Resource Issues
```bash
# Check if pods are hitting resource limits
kubectl top pods -l app=sample-app

# Look for OOMKilled events
kubectl get events --sort-by='.lastTimestamp' | grep OOM
```

## Cleanup

```bash
# Remove all resources
kubectl delete -f k8s-manifests.yaml
kubectl delete -f load-generator.yaml

# Remove the Docker image
docker rmi sample-app:latest
```

## Additional Notes

- The setup uses `imagePullPolicy: Never` for local development
- The Datadog API key is stored as a Kubernetes secret
- Resource limits are set conservatively and can be adjusted based on your needs
- The collector includes both the Datadog exporter and connector for comprehensive data forwarding
- Load balancer service is included for external access if needed # meli-setup-2025
