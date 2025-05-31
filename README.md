# K8s AI Infrastructure Healer

Advanced AI-powered Kubernetes infrastructure monitoring and auto-healing system that detects and fixes issues Kubernetes doesn't see.

## Overview

K8s AI Infrastructure Healer is a comprehensive monitoring and auto-healing solution that goes beyond standard Kubernetes health checks. It uses AI algorithms to predict failures 24-72 hours in advance and automatically remediate infrastructure issues before they impact your applications.

### Key Features

- **Predictive Intelligence**: Forecasts resource exhaustion and failures up to 72 hours ahead
- **Auto-Healing**: Automatically fixes network issues, disk space problems, and stuck containers
- **Advanced Diagnostics**: Detects problems that Kubernetes health checks miss
- **Memory Leak Detection**: Identifies and predicts memory leaks with time-to-failure estimates
- **Web Dashboard**: Real-time monitoring with REST API integration
- **Zero Dependencies**: Works with standard Kubernetes API only

### What Problems Does It Solve?

1. **Stuck Containers**: Detects containers that pass health checks but are unresponsive
2. **Network Connectivity Issues**: Identifies and fixes internal cluster network problems
3. **Disk Space Management**: Monitors and automatically cleans /tmp directories
4. **Memory Leaks**: Predicts memory exhaustion before it happens
5. **Performance Degradation**: Detects gradual performance decline over time
6. **Restart Loops**: Analyzes restart patterns to prevent crash loops

## Quick Start

### Prerequisites

- Kubernetes cluster (version 1.20+)
- kubectl configured for your cluster
- Metrics Server enabled in your cluster

### One-Command Installation

```bash
kubectl apply -f https://raw.githubusercontent.com/Pavel-P09/k8s-ai-healer/main/deployments/install.yaml
```

### Verify Installation

```bash
# Check if healer is running
kubectl get pods -n healer-system

# View logs
kubectl logs -f deployment/k8s-healer -n healer-system

# Access web dashboard
kubectl port-forward svc/k8s-healer 8080:8080 -n healer-system
```

Then open http://localhost:8080 in your browser.

## Installation Options

### Option 1: Deploy to Kubernetes (Recommended)

1. **Create the namespace:**
```bash
kubectl create namespace healer-system
```

2. **Create RBAC permissions:**
```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: k8s-healer
  namespace: healer-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: k8s-healer
rules:
- apiGroups: [""]
  resources: ["pods", "pods/exec", "pods/log", "events", "nodes"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: k8s-healer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: k8s-healer
subjects:
- kind: ServiceAccount
  name: k8s-healer
  namespace: healer-system
EOF
```

3. **Deploy the healer:**
```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k8s-healer
  namespace: healer-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: k8s-healer
  template:
    metadata:
      labels:
        app: k8s-healer
    spec:
      serviceAccountName: k8s-healer
      containers:
      - name: healer
        image: pavel09/k8s-ai-healer:latest
        ports:
        - containerPort: 8080
        env:
        - name: HEALER_PORT
          value: "8080"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: k8s-healer
  namespace: healer-system
spec:
  selector:
    app: k8s-healer
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP
EOF
```

### Option 2: Build from Source

1. **Clone the repository:**
```bash
git clone https://github.com/Pavel-P09/k8s-ai-healer.git
cd k8s-ai-healer
```

2. **Build the binary:**
```bash
go mod download
go build -o bin/healer cmd/healer/main.go
```

3. **Run locally:**
```bash
./bin/healer
```

### Option 3: Run with Docker

1. **Build Docker image:**
```bash
docker build -t k8s-ai-healer .
```

2. **Run container:**
```bash
docker run -v ~/.kube/config:/root/.kube/config k8s-ai-healer
```

## Configuration

### Environment Variables

- `HEALER_PORT`: API server port (default: 8080)
- `HEALER_DRY_RUN`: Enable dry-run mode (default: false)
- `HEALER_LOG_LEVEL`: Log level (default: info)
- `HEALER_CHECK_INTERVAL`: Check interval in seconds (default: 30)

### Example Configuration

```bash
export HEALER_PORT=9090
export HEALER_DRY_RUN=true
export HEALER_LOG_LEVEL=debug
./bin/healer
```

## API Reference

The healer exposes a REST API for integration with external monitoring systems.

### Health Check

```bash
GET /health
```

Response:
```json
{
  "status": "UP",
  "timestamp": "2024-01-01T12:00:00Z",
  "service": "k8s-ai-healer",
  "version": "4.0"
}
```

### System Status

```bash
GET /status
```

Response:
```json
{
  "status": "ACTIVE",
  "timestamp": "2024-01-01T12:00:00Z",
  "total_actions": 42,
  "system_health": "HEALTHY",
  "recent_actions": [...]
}
```

### Healing Actions History

```bash
GET /actions
```

Response:
```json
{
  "total_actions": 42,
  "actions": [
    {
      "ActionType": "RESTART_POD_NETWORK",
      "PodName": "web-app-123",
      "Namespace": "default",
      "Status": "COMPLETED",
      "Timestamp": "2024-01-01T12:00:00Z",
      "Result": "Pod restarted successfully"
    }
  ]
}
```

## How It Works

### Detection Algorithm

1. **Resource Monitoring**: Collects CPU, memory, and disk metrics every 30 seconds
2. **Trend Analysis**: Uses linear regression to detect resource growth patterns
3. **Predictive Modeling**: Forecasts failures 24-72 hours in advance using AI algorithms
4. **Pattern Recognition**: Identifies stuck containers, restart loops, and performance issues

### Auto-Healing Process

1. **Issue Detection**: AI algorithms identify infrastructure problems
2. **Action Selection**: Chooses appropriate remediation based on issue type
3. **Safe Execution**: Performs healing with safety checks and limits
4. **Verification**: Confirms that the action resolved the issue
5. **Logging**: Records all actions for audit and analysis

### Safety Features

- **Dry-run mode** for testing without making changes
- **Action limits** to prevent infinite loops (max 3 actions per pod)
- **Graceful restarts** with proper termination handling
- **Rollback capability** for failed healing attempts

## Examples

### Example 1: Memory Leak Detection

When the healer detects a memory leak:

```
K8s AI Healer v4.0 - COMPLETE SYSTEM WITH API
Connected to cluster
AI Monitoring started - COMPLETE SYSTEM ACTIVE

SMART PREDICTIONS & FORECASTS
Pod: default/memory-leak-app - Risk: CRITICAL (Score: 85.0, 95% confidence)
  PREDICTION: Failure in 18.5 hours (Memory leak)
  Memory leak: +3.2%/hour
  AI Action: RESTART_POD_URGENT

AUTO-HEALING ACTIONS
RESTART_POD_NETWORK: default/memory-leak-app/app
  Restarted pod due to memory leak prediction
  Result: Pod restarted successfully
```

### Example 2: Network Connectivity Issues

When network problems are detected:

```
CONTAINER HEALTH CHECKS
Container: default/web-app/app - WARNING
  Network Connectivity: Internal cluster connectivity issues
     Actions: [CHECK_NETWORK RESTART_POD]

AUTO-HEALING ACTIONS
FIX_NETWORK: default/web-app/app
  Fixing network connectivity
  Result: Network FAIL; Pod restarted successfully
```

### Example 3: Disk Space Management

When /tmp directory fills up:

```
CONTAINER HEALTH CHECKS
Container: default/data-processor/app - CRITICAL
  /tmp Directory: /tmp directory 95% full
     Actions: [CLEANUP_TMP]

AUTO-HEALING ACTIONS
CLEANUP_TMP: default/data-processor/app
  Cleaning up /tmp directory
  Result: Cleanup executed; 500MB freed
```

## Monitoring Integration

### Prometheus Metrics

The healer can expose Prometheus-compatible metrics:

```
# Add to your prometheus.yml
- job_name: 'k8s-healer'
  static_configs:
  - targets: ['k8s-healer.healer-system:8080']
```

### Grafana Dashboard

Create a dashboard to visualize:
- System health over time
- Healing action frequency
- Pod restart patterns
- Resource usage trends

## Troubleshooting

### Common Issues

1. **Metrics not available**: Ensure Metrics Server is installed
```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

2. **Permission denied**: Check RBAC permissions
```bash
kubectl auth can-i get pods --as=system:serviceaccount:healer-system:k8s-healer
```

3. **API server not responding**: Check if port 8080 is available
```bash
kubectl port-forward svc/k8s-healer 8080:8080 -n healer-system
```

### Debug Mode

Enable debug logging:
```bash
export HEALER_LOG_LEVEL=debug
./bin/healer
```

## Contributing

We welcome contributions! Please follow these steps:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

### Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/k8s-ai-healer.git
cd k8s-ai-healer

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o bin/healer cmd/healer/main.go

# Run locally
./bin/healer
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues and questions:
- Create an issue on GitHub
- Check the troubleshooting section
- Review the API documentation

## Roadmap

Planned features:
- [ ] Support for custom healing actions
- [ ] Integration with external alerting systems
- [ ] Machine learning model improvements
- [ ] Multi-cluster support
- [ ] Advanced grafana dashboards

---

**Star this repository if it helped you!**

Made with ❤️ for the Kubernetes community
