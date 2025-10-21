# Quick Deployment Guide

## Prerequisites

- Kubernetes 1.29+ or OpenShift 4.16+
- `kubectl` or `oc` CLI
- Cluster admin permissions

## Option 1: Deploy from Source (Kustomize)

### Deploy Operator

```bash
# Clone the repository
git clone https://github.com/rajinator/installplan-approver-operator.git
cd installplan-approver-operator

# Deploy CRDs, RBAC, and operator
kubectl apply -k config/default

# Or for OpenShift
oc apply -k config/default
```

### Verify Deployment

```bash
# Check namespace
kubectl get namespace iplan-approver-system

# Check operator pod
kubectl get pods -n iplan-approver-system

# Expected output:
# NAME                                                  READY   STATUS    RESTARTS   AGE
# iplan-approver-controller-manager-xxxxxxxxxx-xxxxx   1/1     Running   0          30s

# Check CRD
kubectl get crd installplanapprovers.operators.bapu.cloud

# Or using shortname
kubectl api-resources | grep ipa
```

### Create InstallPlanApprover Resource

```bash
# Apply a sample configuration
kubectl apply -f config/samples/operators_v1alpha1_installplanapprover.yaml

# Or create custom configuration
cat <<EOF | kubectl apply -f -
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: auto-approver
  namespace: iplan-approver-system
spec:
  autoApprove: true
  targetNamespaces:
    - openshift-operators
    - cert-manager
EOF

# Check status using shortname
kubectl get ipa -n iplan-approver-system
```

---

## Option 2: Deploy from GitHub (Remote Kustomize)

### Deploy Directly from GitHub

```bash
# Deploy operator (replace 'main' with specific version tag if needed)
kubectl apply -k github.com/rajinator/installplan-approver-operator/config/default?ref=main

# Or for a specific version/tag
kubectl apply -k github.com/rajinator/installplan-approver-operator/config/default?ref=v0.1.0
```

### Create Configuration

```bash
# Download and customize sample
curl -sL https://raw.githubusercontent.com/rajinator/installplan-approver-operator/main/config/samples/operators_v1alpha1_installplanapprover.yaml | \
  kubectl apply -f -
```

---

## Option 3: Deploy with Custom Image

If you've built a custom image:

```bash
cd installplan-approver-operator

# Build image
export IMG=ghcr.io/rajinator/installplan-approver-operator:latest
make docker-build

# Push image  
make docker-push

# Deploy with custom image
cd config/manager
kustomize edit set image controller=${IMG}
cd ../..
kubectl apply -k config/default
```

---

## Components Deployed

The default kustomization deploys:

| Component | Namespace | Description |
|-----------|-----------|-------------|
| **Namespace** | `iplan-approver-system` | Operator namespace |
| **CRD** | Cluster-wide | `installplanapprovers.operators.bapu.cloud` |
| **ServiceAccount** | `iplan-approver-system` | `iplan-approver-controller-manager` |
| **ClusterRole** | Cluster-wide | RBAC for operator |
| **ClusterRoleBinding** | Cluster-wide | Binds SA to ClusterRole |
| **Deployment** | `iplan-approver-system` | Operator controller |
| **Service** | `iplan-approver-system` | Metrics endpoint |

---

## Configuration Examples

### Example 1: Auto-approve in Specific Namespaces

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: selective-approver
  namespace: iplan-approver-system
spec:
  autoApprove: true
  targetNamespaces:
    - openshift-operators
    - cert-manager
    - gitlab-runner
```

### Example 2: Auto-approve Specific Operators Only

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: filtered-approver
  namespace: iplan-approver-system
spec:
  autoApprove: true
  targetNamespaces:
    - openshift-operators
  operatorNames:
    - cert-manager
    - gitlab-runner-operator
```

### Example 3: Approve All Namespaces

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: global-approver
  namespace: iplan-approver-system
spec:
  autoApprove: true
  # Empty targetNamespaces means all namespaces
```

---

## Monitoring

### Check Operator Logs

```bash
# Get operator pod name
POD=$(kubectl get pods -n iplan-approver-system -l control-plane=controller-manager -o jsonpath='{.items[0].metadata.name}')

# View logs
kubectl logs -f $POD -n iplan-approver-system

# Or using stern (if installed)
stern -n iplan-approver-system controller-manager
```

### Check InstallPlanApprover Status

```bash
# Using full name
kubectl get installplanapprovers -n iplan-approver-system -o yaml

# Using shortname
kubectl describe ipa -n iplan-approver-system

# Check status fields
kubectl get ipa auto-approver -n iplan-approver-system -o jsonpath='{.status}'
```

### View Metrics

```bash
# Port-forward to metrics endpoint
kubectl port-forward -n iplan-approver-system svc/iplan-approver-controller-manager-metrics-service 8443:8443

# Query metrics (in another terminal)
curl -k https://localhost:8443/metrics
```

---

## Uninstall

### Remove InstallPlanApprovers

```bash
# Delete all InstallPlanApprover resources
kubectl delete ipa --all -n iplan-approver-system

# Or delete specific resource
kubectl delete ipa auto-approver -n iplan-approver-system
```

### Remove Operator

```bash
# From source
kubectl delete -k config/default

# Or from GitHub
kubectl delete -k github.com/rajinator/installplan-approver-operator/config/default?ref=main

# Verify removal
kubectl get namespace iplan-approver-system
# Should show: Error from server (NotFound)
```

---

## Troubleshooting

### Operator Not Starting

```bash
# Check pod status
kubectl get pods -n iplan-approver-system

# Check events
kubectl get events -n iplan-approver-system --sort-by='.lastTimestamp'

# Check pod logs
kubectl logs -n iplan-approver-system deployment/iplan-approver-controller-manager
```

### InstallPlans Not Being Approved

1. **Check operator logs:**
   ```bash
   kubectl logs -n iplan-approver-system -l control-plane=controller-manager -f
   ```

2. **Verify InstallPlanApprover exists:**
   ```bash
   kubectl get ipa -A
   ```

3. **Check if autoApprove is enabled:**
   ```bash
   kubectl get ipa -n iplan-approver-system -o jsonpath='{.items[*].spec.autoApprove}'
   ```

4. **Check operator RBAC:**
   ```bash
   kubectl get clusterrole iplan-approver-manager-role -o yaml
   ```

### Version Mismatch Issues

If InstallPlans aren't being approved due to version mismatch:

```bash
# Check operator logs for version matching messages
kubectl logs -n iplan-approver-system -l control-plane=controller-manager | grep "CSV"

# Check Subscription startingCSV
kubectl get subscription -n <namespace> <name> -o jsonpath='{.spec.startingCSV}'

# Check InstallPlan CSV
kubectl get installplan -n <namespace> <name> -o jsonpath='{.spec.clusterServiceVersionNames}'
```

---

## OpenShift-Specific Notes

### Using Internal Registry

```bash
# Build and push to OpenShift internal registry
oc new-build --name=installplan-approver --binary --strategy=docker -n openshift-operators
oc start-build installplan-approver --from-dir=. --follow -n openshift-operators

# Get image reference
IMG=$(oc get is installplan-approver -n openshift-operators -o jsonpath='{.status.dockerImageRepository}')

# Deploy with internal image
cd config/manager
kustomize edit set image controller=${IMG}:latest
cd ../..
oc apply -k config/default
```

### SecurityContextConstraints

The operator runs with restricted permissions by default. If you encounter SCC issues:

```bash
# Check pod security
oc get pod -n iplan-approver-system -o jsonpath='{.items[*].metadata.annotations.openshift\.io/scc}'

# The operator should work with 'restricted' SCC
```

---

## Next Steps

- See [README.md](README.md) for detailed documentation
- See [QUICKSTART.md](QUICKSTART.md) for testing guide

---

## Quick Reference

```bash
# Deploy
kubectl apply -k config/default

# Create resource
kubectl apply -f config/samples/operators_v1alpha1_installplanapprover.yaml

# Check status
kubectl get ipa -A

# View logs
kubectl logs -n iplan-approver-system -l control-plane=controller-manager -f

# Uninstall
kubectl delete -k config/default
```

