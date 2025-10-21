# Installation Guide

Complete installation guide for the InstallPlan Approver Operator.

## Prerequisites

### Required
- Kubernetes v1.30+ or OpenShift v4.16+
- OLM (Operator Lifecycle Manager) installed
- `kubectl` or `oc` CLI tool

### For Development
- Go 1.24.3+
- Operator SDK v1.41.1+
- Docker or Podman
- Make

## Container Images

AMD64 images are available with the latest tag right now on GitHub Container Registry (GHCR)
TO-DO Multi-architecture images will be available on GitHub Container Registry (GHCR):

```
ghcr.io/rajinator/installplan-approver-operator:v0.1.0  # Stable release
ghcr.io/rajinator/installplan-approver-operator:latest  # Development (amd64 only)
```

**Architecture support:**
- Release tags (`v*`): `linux/amd64`, `linux/arm64`
- Development (`latest`): `linux/amd64` only

**No authentication required** - all images are public.

## Installation Methods

### Method 1: Quick Install (Recommended)

Install directly from the operator's GitHub repository:

```bash
kubectl apply -k 'github.com/rajinator/installplan-approver-operator/config/default?ref=v0.1.0'
```

**For OpenShift (quote URL to avoid zsh glob issues):**
```bash
oc apply -k 'github.com/rajinator/installplan-approver-operator/config/default?ref=v0.1.0'
```

**What this does:**
- Creates `iplan-approver-system` namespace
- Installs CRDs
- Deploys operator with RBAC
- Pulls image from GHCR

### Method 2: Clone and Deploy

Clone the repository and deploy:

```bash
# Clone repository
git clone https://github.com/rajinator/installplan-approver-operator.git
cd installplan-approver-operator

# Checkout specific version
git checkout v0.1.0

# Deploy using make
make deploy IMG=ghcr.io/rajinator/installplan-approver-operator:v0.1.0
```

### Method 3: Development/Latest Build

Use the latest development build (amd64 only):

```bash
kubectl apply -k github.com/rajinator/installplan-approver-operator/config/default
```

**Warning:** Development builds may be unstable.

### Method 4: Custom Image

Build and deploy your own image:

```bash
# Clone repository
git clone https://github.com/rajinator/installplan-approver-operator.git
cd installplan-approver-operator

# Build image
make docker-build IMG=<your-registry>/installplan-approver-operator:latest

# Push image
make docker-push IMG=<your-registry>/installplan-approver-operator:latest

# Deploy
make deploy IMG=<your-registry>/installplan-approver-operator:latest
```

## Verification

### Check Deployment

```bash
# Check namespace
kubectl get namespace iplan-approver-system

# Check deployment
kubectl get deployment -n iplan-approver-system

# Expected output:
# NAME                                  READY   UP-TO-DATE   AVAILABLE   AGE
# iplan-approver-controller-manager     1/1     1            1           2m
```

### Check Pods

```bash
kubectl get pods -n iplan-approver-system

# Expected output:
# NAME                                                READY   STATUS    RESTARTS   AGE
# iplan-approver-controller-manager-xxxxxxxxxx-xxxxx   1/1     Running   0          2m
```

### Check Logs

```bash
kubectl logs -n iplan-approver-system -l control-plane=controller-manager -f
```

**Healthy logs should show:**
```
INFO  Starting Controller	controller=installplanapprover
INFO  Starting workers	controller=installplanapprover worker count=1
```

### Check CRD

```bash
kubectl get crd installplanapprovers.operators.bapu.cloud

# Expected output:
# NAME                                        CREATED AT
# installplanapprovers.operators.bapu.cloud   2025-10-21T12:34:56Z
```

## Post-Installation

### Create InstallPlanApprover Resource

After installation, create an `InstallPlanApprover` CR:

```bash
kubectl apply -f - <<EOF
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: my-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces:
    - cert-manager
    - monitoring
  operatorNames:
    - cert-manager
    - prometheus
EOF
```

### Verify Approver

```bash
kubectl get installplanapprovers -A
kubectl describe installplanapprover my-approver -n operators
```

## Upgrade

### Upgrade to New Version

```bash
# Using kubectl
kubectl apply -k 'github.com/rajinator/installplan-approver-operator/config/default?ref=v0.2.0'

# Or using make
make deploy IMG=ghcr.io/rajinator/installplan-approver-operator:v0.2.0
```

The operator will:
- Update deployment with new image
- Preserve existing InstallPlanApprover CRs
- Continue processing without downtime

### Rollback

```bash
# Rollback to previous version
kubectl rollout undo deployment/iplan-approver-controller-manager -n iplan-approver-system

# Or redeploy old version
kubectl apply -k 'github.com/rajinator/installplan-approver-operator/config/default?ref=v0.1.0'
```

## Uninstallation

### Remove Operator

```bash
# Using make
make undeploy

# Or using kubectl
kubectl delete -k 'github.com/rajinator/installplan-approver-operator/config/default?ref=v0.1.0'
```

### Remove CRDs

```bash
# Using make
make uninstall

# Or manually
kubectl delete crd installplanapprovers.operators.bapu.cloud
```

**Warning:** Deleting CRDs will also delete all InstallPlanApprover resources.

### Clean Up Resources

```bash
# Delete namespace
kubectl delete namespace iplan-approver-system

# Verify cleanup
kubectl get installplanapprovers -A
```

## Advanced Installation

### Custom Namespace

Deploy to a custom namespace:

```bash
# Clone repository
git clone https://github.com/rajinator/installplan-approver-operator.git
cd installplan-approver-operator

# Edit config/default/kustomization.yaml
# Change namespace to your desired namespace

# Deploy
make deploy IMG=ghcr.io/rajinator/installplan-approver-operator:v0.1.0
```

### Air-Gapped Installation

For air-gapped environments:

```bash
# 1. Pull image on internet-connected machine
docker pull ghcr.io/rajinator/installplan-approver-operator:v0.1.0

# 2. Save image
docker save ghcr.io/rajinator/installplan-approver-operator:v0.1.0 -o installplan-approver-operator.tar

# 3. Transfer to air-gapped environment

# 4. Load image
docker load -i installplan-approver-operator.tar

# 5. Tag for local registry
docker tag ghcr.io/rajinator/installplan-approver-operator:v0.1.0 <local-registry>/installplan-approver-operator:v0.1.0

# 6. Push to local registry
docker push <local-registry>/installplan-approver-operator:v0.1.0

# 7. Deploy
make deploy IMG=<local-registry>/installplan-approver-operator:v0.1.0
```

### Helm (Future)

Helm chart is planned for future releases.

## Troubleshooting Installation

### Image Pull Errors

```bash
# Check image pull status
kubectl describe pod -n iplan-approver-system

# Common issues:
# 1. Wrong image tag
# 2. Registry authentication (shouldn't be needed for GHCR public images)
# 3. Network issues
```

### RBAC Errors

```bash
# Check ClusterRole
kubectl get clusterrole | grep iplan-approver

# Check ClusterRoleBinding
kubectl get clusterrolebinding | grep iplan-approver

# Verify ServiceAccount
kubectl get sa -n iplan-approver-system
```

### CRD Already Exists

```bash
# If CRD already exists from previous installation
kubectl delete crd installplanapprovers.operators.bapu.cloud

# Then retry installation
```

For more troubleshooting, see [TROUBLESHOOTING.md](TROUBLESHOOTING.md).

