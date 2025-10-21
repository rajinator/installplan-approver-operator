# Quick Start Guide

This guide will help you get the InstallPlan Approver Operator running in under 5 minutes.

## Prerequisites

- Kubernetes cluster (kind, minikube, or any K8s cluster)
- kubectl configured
- operator-sdk installed
- Go 1.24.3+ (for local development)

## Option 1: Automated Quick Start

Use the provided test script:

```bash
# Check prerequisites
./test-operator.sh check

# Install CRDs
./test-operator.sh install

# Create sample InstallPlanApprover
./test-operator.sh create-sample

# Run operator locally
./test-operator.sh run
```

In another terminal:

```bash
# Watch for InstallPlans
./test-operator.sh watch

# Check operator status
./test-operator.sh status
```

## Option 2: Manual Quick Start

### Step 1: Install CRDs

```bash
make install
```

### Step 2: Run Operator Locally

In one terminal:

```bash
make run
```

### Step 3: Create Sample InstallPlanApprover

In another terminal:

```bash
kubectl apply -f config/samples/operators_v1alpha1_installplanapprover.yaml
```

### Step 4: Verify

Check the InstallPlanApprover:

```bash
kubectl get installplanapprovers
kubectl get installplanapprovers installplanapprover-sample -o yaml
```

## Testing with OLM

If you have OLM installed, you can test with a real InstallPlan:

### Step 1: Install OLM (if not already installed)

```bash
operator-sdk olm install
```

### Step 2: Create a Subscription with Manual Approval

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: test-operators
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: test-operators
  namespace: test-operators
spec:
  targetNamespaces:
    - test-operators
---
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: operatorhubio-catalog
  namespace: olm
spec:
  sourceType: grpc
  image: quay.io/operatorhubio/catalog:latest
  displayName: Community Operators
  publisher: OperatorHub.io
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: prometheus
  namespace: test-operators
spec:
  channel: beta
  name: prometheus
  source: operatorhubio-catalog
  sourceNamespace: olm
  installPlanApproval: Manual
EOF
```

### Step 3: Update InstallPlanApprover to Watch test-operators

```bash
cat <<EOF | kubectl apply -f -
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: installplanapprover-sample
spec:
  autoApprove: true
  targetNamespaces:
    - test-operators
EOF
```

### Step 4: Watch the Magic Happen

Watch InstallPlans get automatically approved:

```bash
kubectl get installplans -n test-operators -w
```

Check the operator status:

```bash
kubectl get installplanapprovers installplanapprover-sample -o yaml
```

## Testing without OLM

If you don't have OLM, you can create a mock InstallPlan:

```bash
# Create test namespace
kubectl create namespace test-operators

# Create mock InstallPlan (this won't actually install anything)
cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: InstallPlan
metadata:
  name: test-installplan
  namespace: test-operators
spec:
  approved: false
  clusterServiceVersionNames:
    - test-operator.v1.0.0
EOF

# Watch it get approved
kubectl get installplan test-installplan -n test-operators -w
```

## Sample Configurations

### Approve All InstallPlans in All Namespaces

```bash
kubectl apply -f config/samples/installplanapprover-all-namespaces.yaml
```

### Approve Only Specific Operators

```bash
kubectl apply -f config/samples/installplanapprover-filtered.yaml
```

### Approve in Specific Namespaces

```bash
kubectl apply -f config/samples/operators_v1alpha1_installplanapprover.yaml
```

## Cleanup

```bash
# Delete sample InstallPlanApprovers
kubectl delete installplanapprovers --all

# Uninstall CRDs
make uninstall

# Clean up test resources
kubectl delete namespace test-operators
```

## Troubleshooting

### Operator Not Approving InstallPlans

1. Check operator logs:
   ```bash
   # If running locally
   # Check the terminal where you ran 'make run'
   
   # If deployed to cluster
   kubectl logs -n installplan-approver-operator-system deployment/installplan-approver-operator-controller-manager
   ```

2. Check if InstallPlan CRD exists:
   ```bash
   kubectl get crd installplans.operators.coreos.com
   ```

3. Verify InstallPlanApprover is created:
   ```bash
   kubectl get installplanapprovers -A
   ```

### OLM Not Installed

If you see errors about InstallPlan CRD not found, install OLM:

```bash
operator-sdk olm install
```

## Next Steps

- Read the full [README.md](README.md) for detailed documentation
- Deploy to cluster: See README.md section "Deploy to Cluster"
- Customize for your use case: Edit the controller logic in `internal/controller/`

## Quick Commands Reference

```bash
# Install CRDs
make install

# Run locally
make run

# Build
make build

# Generate manifests
make manifests

# Generate code
make generate

# Run tests
make test

# Deploy to cluster
make deploy IMG=<your-image>

# Uninstall
make uninstall
```

## Performance Notes

This operator uses:
- **Informers**: For efficient watching of resources
- **Listers**: For cached access to resources
- **Reconciliation Loop**: Runs every 30 seconds to check for new InstallPlans
- **Event-driven**: Also reconciles immediately when resources change

This design ensures minimal API server load and quick response times.

