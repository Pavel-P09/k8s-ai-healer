#!/bin/bash

echo "Building K8s Healer Docker image..."
docker build -t k8s-healer:latest .

echo "Deploying to Kubernetes..."
kubectl apply -f deployments/namespace.yaml
kubectl apply -f deployments/rbac.yaml
kubectl apply -f deployments/deployment.yaml

echo "Waiting for deployment..."
kubectl wait --for=condition=available --timeout=60s deployment/k8s-healer -n healer-system

echo "Deployment complete!"
echo "Check status: kubectl get pods -n healer-system"
echo "View logs: kubectl logs -f deployment/k8s-healer -n healer-system"
