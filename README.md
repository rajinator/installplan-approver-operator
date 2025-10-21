# InstallPlan Approver Operator

A Kubernetes operator that automatically approves OLM (Operator Lifecycle Manager) InstallPlans, enabling GitOps-based operator version control at scale.

**Assisted by: Cursor with claude-4.5-sonnet**

## Why This Operator?

### The Problem

When using OLM with `installPlanApproval: Manual`, you gain precise control over operator versions—critical for production environments and GitOps workflows. However, this creates an operational burden:

- **Manual Intervention Required**: Each InstallPlan needs manual approval (`oc patch` or `kubectl patch`)
- **No GitOps Integration**: Manual approvals don't fit into automated GitOps pipelines
- **Scale Issues**: Managing 10s or 100s of operators across multiple clusters becomes unmanageable
- **Job/CronJob Race Conditions**: Using CronJobs to approve InstallPlans can miss newly created plans between runs or create approval conflicts

### The Solution

This operator automates InstallPlan approvals while maintaining the benefits of `installPlanApproval: Manual`:

✅ **Version Control**: Your Subscriptions (in Git) define exact operator versions  
✅ **GitOps Friendly**: Fully automated approval process  
✅ **Event-Driven**: Uses informers to react immediately when InstallPlans are created (no polling, no race conditions)  
✅ **Selective Approval**: Filter by namespace, operator name, or labels (future)  
✅ **Audit Trail**: Track all approvals via operator status and logs  
✅ **Efficiency at Scale**: Handles hundreds of operators across multiple namespaces  

**vs. CronJob approach:** Unlike periodic jobs that poll for InstallPlans (missing new plans between runs or creating race conditions), this operator watches in real-time using Kubernetes informers—approvals happen within milliseconds of InstallPlan creation.  

### Use Case: GitOps + Pinned Versions

```yaml
# In Git: operators/prometheus-subscription.yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: prometheus-operator
  namespace: monitoring
spec:
  channel: stable
  name: prometheus
  source: operatorhubio-catalog
  sourceNamespace: olm
  installPlanApproval: Manual          # ← Pin versions via Git
  startingCSV: prometheus-operator.v0.68.0  # ← Exact version control
```

**Without this operator:** Manual `kubectl patch` required for every InstallPlan  
**With this operator:** Automatic approval for the pinned version only (v0.68.0), upgrades still require Git updates

**Important:** The operator only approves InstallPlans where the CSV version **exactly matches** the Subscription's `startingCSV`. This prevents accidental auto-approval of upgrades, preserving true version control.

## Overview

The InstallPlan Approver Operator watches for InstallPlans in your Kubernetes cluster and automatically approves them based on your configuration. This eliminates manual intervention while preserving version control through your Subscription manifests.

### Key Features

- **GitOps Integration**: Works seamlessly with ArgoCD, Flux, and other GitOps tools
- **Version Control**: Maintain operator versions in Git via Subscriptions with `installPlanApproval: Manual`
- **Version-Matched Approval**: Only approves InstallPlans that match the Subscription's `startingCSV` - prevents unintended upgrades
- **Automatic Approval**: Approve InstallPlans automatically based on your policy
- **Namespace Filtering**: Target specific namespaces or watch all namespaces
- **Operator Filtering**: Approve only specific operators (allowlist)
- **Efficient at Scale**: Uses informers and listers for minimal API server load
- **Audit Trail**: Track approval count, timestamps, and history in operator status

## Prerequisites

- Kubernetes v1.30+ or OpenShift cluster v4.16+
- OLM (Operator Lifecycle Manager) installed in the cluster

**For Development:**
- Go 1.24.3+
- Operator SDK v1.41.1+

## Installation

### Pre-built Container Images

Multi-architecture images (amd64, arm64) are available on GitHub Container Registry:

```bash
ghcr.io/rajinator/installplan-approver-operator:latest
ghcr.io/rajinator/installplan-approver-operator:v0.1.0  # Specific version
```

**No authentication required** - all images are public.

### Quick Install

**Using kustomize:**
```bash
kubectl apply -k github.com/rajinator/installplan-approver-operator/config/default?ref=v1.0.0
```

**Using operator image directly:**
```bash
make deploy IMG=ghcr.io/rajinator/installplan-approver-operator:v1.0.0
```

**Verify installation:**
```bash
kubectl get deployment -n iplan-approver-system
kubectl logs -n iplan-approver-system -l control-plane=controller-manager -f
```

For detailed installation options, see [DEPLOY.md](DEPLOY.md).

## Quick Start

### Local Development and Testing

1. **Install CRDs**:
   ```bash
   make install
   ```

2. **Run the operator locally** (against your current kubeconfig context):
   ```bash
   make run
   ```

3. **In another terminal, create a sample InstallPlanApprover**:
   ```bash
   kubectl apply -f config/samples/operators_v1alpha1_installplanapprover.yaml
   ```

4. **Check the operator logs** to see it watching for InstallPlans:
   The operator will automatically approve any unapproved InstallPlans in the specified namespaces.

5. **Check the status**:
   ```bash
   kubectl get installplanapprovers installplanapprover-sample -o yaml
   ```

### Deploy to Cluster

1. **Build and push the image**:
   ```bash
   make docker-build docker-push IMG=<your-registry>/installplanapprover-operator:latest
   ```

2. **Deploy to cluster**:
   ```bash
   make deploy IMG=<your-registry>/installplanapprover-operator:latest
   ```

3. **Create a sample InstallPlanApprover**:
   ```bash
   kubectl apply -f config/samples/operators_v1alpha1_installplanapprover.yaml
   ```

## Configuration

### InstallPlanApprover Spec

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: installplanapprover-sample
spec:
  # Enable automatic approval of InstallPlans
  autoApprove: true
  
  # Target specific namespaces (empty means all namespaces)
  targetNamespaces:
    - cert-manager
    - gitlab-runner-operator
  
  # Optionally specify operator names to approve (empty means all operators)
  operatorNames:
    - cert-manager
    - gitlab-runner-operator
```

### Fields

- **autoApprove** (bool): Enable or disable automatic approval. Default: `true`
- **targetNamespaces** ([]string): List of namespaces to watch. Empty means all namespaces.
- **operatorNames** ([]string): List of operator names to approve. Empty means approve all operators.

### InstallPlanApprover Status

The operator updates the status with:
- **approvedCount** (int32): Total number of InstallPlans approved
- **lastApprovedPlan** (string): Name and namespace of the last approved InstallPlan
- **lastApprovedTime** (metav1.Time): Timestamp of the last approval

## Architecture

### Components

1. **Controller**: Reconciles InstallPlanApprover resources and processes InstallPlans
2. **Informers**: Efficiently watch InstallPlans and InstallPlanApprover resources
3. **Listers**: Provide cached access to resources, reducing API server load

### How It Works

1. The operator watches for InstallPlanApprover custom resources
2. When an InstallPlanApprover is created/updated, the operator:
   - Lists InstallPlans in the specified namespaces (or all namespaces)
   - Filters by operator names if specified
   - Approves unapproved InstallPlans by setting `spec.approved: true`
   - Updates the status with approval count and timestamps
3. The operator also watches InstallPlan resources and triggers reconciliation when new InstallPlans are created

### Reconciliation Loop

- The operator reconciles every 30 seconds to check for new InstallPlans
- It also reconciles immediately when:
  - An InstallPlanApprover is created/updated/deleted
  - An InstallPlan is created/updated (via informer watch)

## Testing

### Unit Tests

Run the unit tests:
```bash
make test
```

### Integration Testing

1. Install OLM (if not already installed):
   ```bash
   operator-sdk olm install
   ```

2. Install a test operator with manual approval:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: v1
   kind: Namespace
   metadata:
     name: test-operators
   ---
   apiVersion: operators.coreos.com/v1alpha1
   kind: Subscription
   metadata:
     name: cert-manager
     namespace: test-operators
   spec:
     channel: stable
     name: cert-manager
     source: community-operators
     sourceNamespace: olm
     installPlanApproval: Manual
   EOF
   ```

3. Create an InstallPlanApprover targeting the test namespace:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: operators.bapu.cloud/v1alpha1
   kind: InstallPlanApprover
   metadata:
     name: test-approver
   spec:
     autoApprove: true
     targetNamespaces:
       - test-operators
   EOF
   ```

4. Watch the InstallPlan get approved:
   ```bash
   kubectl get installplans -n test-operators -w
   ```

## Container Images

This operator uses **Red Hat Universal Base Images (UBI)**

**Current Images:**
- Build: `registry.access.redhat.com/ubi9/go-toolset:latest`
- Runtime: `registry.access.redhat.com/ubi9/ubi-minimal:latest`

✅ **Enterprise ready**  

## Development

### Project Structure

```
.
├── api/v1alpha1/              # API definitions
│   └── installplanapprover_types.go
├── cmd/                       # Main entry point
│   └── main.go
├── config/                    # Deployment manifests
│   ├── crd/                   # CRD definitions
│   ├── manager/               # Manager deployment
│   ├── rbac/                  # RBAC roles and bindings
│   └── samples/               # Sample CRs
├── internal/controller/       # Controller implementation
│   └── installplanapprover_controller.go
└── Makefile                   # Build and deployment targets
```

### Makefile Targets

- `make manifests`: Generate CRD and RBAC manifests
- `make generate`: Generate code (DeepCopy, etc.)
- `make build`: Build the operator binary
- `make test`: Run unit tests
- `make run`: Run the operator locally
- `make install`: Install CRDs to the cluster
- `make uninstall`: Uninstall CRDs from the cluster
- `make deploy`: Deploy the operator to the cluster
- `make undeploy`: Remove the operator from the cluster
- `make docker-build`: Build the Docker image
- `make docker-push`: Push the Docker image

### RBAC Permissions

The operator requires the following permissions:

- **InstallPlanApprover resources**: Full CRUD access
- **InstallPlans (operators.coreos.com)**: Get, List, Watch, Update, Patch
- **Namespaces**: Get, List, Watch (for namespace discovery)

## Troubleshooting

### Operator not approving InstallPlans

1. Check the operator logs:
   ```bash
   kubectl logs -n installplan-approver-operator-system deployment/installplan-approver-operator-controller-manager
   ```

2. Verify the InstallPlanApprover is created:
   ```bash
   kubectl get installplanapprovers
   ```

3. Check if OLM is installed:
   ```bash
   kubectl get crd installplans.operators.coreos.com
   ```

4. Verify RBAC permissions:
   ```bash
   kubectl describe clusterrole installplan-approver-operator-manager-role
   ```

### InstallPlans not appearing

1. Ensure you have a Subscription with `installPlanApproval: Manual`:
   ```bash
   kubectl get subscriptions -A
   ```

2. Check if InstallPlans are created:
   ```bash
   kubectl get installplans -A
   ```

## Cleanup

### Remove the operator

```bash
make undeploy
```

### Uninstall CRDs

```bash
make uninstall
```

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
