#!/bin/bash

echo "🚀 Deploying K8s Healer in PRODUCTION mode (real healing actions)..."

# Build image
docker build -t k8s-healer:production .

# Update deployment with production mode
cat > deployments/deployment-production.yaml << 'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: k8s-healer-production
  namespace: healer-system
  labels:
    app: k8s-healer-production
spec:
  replicas: 1
  selector:
    matchLabels:
      app: k8s-healer-production
  template:
    metadata:
      labels:
        app: k8s-healer-production
    spec:
      serviceAccountName: k8s-healer
      containers:
      - name: healer
        image: k8s-healer:production
        imagePullPolicy: Never
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "100m"
        env:
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: DRY_RUN
          value: "false"
YAML

echo "Applying deployment..."
kubectl apply -f deployments/deployment-production.yaml

echo "✅ Production deployment complete!"
echo "⚠️  WARNING: This will perform REAL healing actions!"
echo "Check logs: kubectl logs -f deployment/k8s-healer-production -n healer-system"
