# InstallPlan Approver Operator

A Kubernetes operator that automatically approves OLM (Operator Lifecycle Manager) InstallPlans, enabling GitOps-based operator version control at scale.

**Assisted by: Cursor with claude-4.5-sonnet**

[![Container Images](https://img.shields.io/badge/images-GHCR-blue)](https://ghcr.io/rajinator/installplan-approver-operator)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Why This Operator?

When using OLM with `installPlanApproval: Manual`, you gain precise control over operator versions, which is critical for production environments and GitOps workflows. However, this creates an operational burden:

- ❌ **Manual Intervention Required**: Each InstallPlan needs manual `kubectl patch`
- ❌ **No GitOps Integration**: Manual approvals don't fit automated pipelines
- ❌ **Scale Issues**: Managing 10s or 100s of operators becomes unmanageable
- ❌ **CronJob Race Conditions**: Periodic polling misses newly created plans

### The Solution

This operator automates InstallPlan approvals while preserving version control:

✅ **Version Control**: Subscriptions (in Git) define exact operator versions  
✅ **GitOps Friendly**: Fully automated approval process  
✅ **Event-Driven**: Reacts immediately when InstallPlans are created (no polling)  
✅ **Version-Matched**: Only approves if CSV matches Subscription's `startingCSV`  
✅ **Audit Trail**: Track all approvals via operator status and logs  
✅ **Efficient**: Handles hundreds of operators with minimal API load  

**vs. CronJob approach:** Unlike periodic jobs that poll for InstallPlans, this operator watches in real-time using Kubernetes informers—approvals happen within milliseconds of InstallPlan creation.

## Quick Example

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
  installPlanApproval: Manual          # ← Pin versions via Git
  startingCSV: prometheus-operator.v0.68.0  # ← Exact version control
```

**Without this operator:** Manual `kubectl patch` required or job/cronjob required

**With this operator:** Automatic approval for v0.68.0 only, upgrades require Git updates

> **Important:** The operator only approves InstallPlans where the CSV version **exactly matches** the Subscription's `startingCSV`. This prevents accidental auto-approval of upgrades.

## Prerequisites

- Kubernetes v1.30+ or OpenShift v4.16+
- OLM (Operator Lifecycle Manager) installed

## Installation

### Quick Install (Recommended)

```bash
kubectl apply -k 'github.com/rajinator/installplan-approver-operator/config/default?ref=v0.1.0'

# For OpenShift (quote to avoid zsh glob issues)
oc apply -k 'github.com/rajinator/installplan-approver-operator/config/default?ref=v0.1.0'
```

**Verify:**
```bash
kubectl get deployment -n iplan-approver-system
kubectl logs -n iplan-approver-system -l control-plane=controller-manager -f
```

### Container Images

Multi-architecture images (amd64, arm64) available on GHCR:

```
ghcr.io/rajinator/installplan-approver-operator:v0.1.0  # Stable release
ghcr.io/rajinator/installplan-approver-operator:latest  # Development (amd64 only)
```

**No authentication required** - all images are public.

For detailed installation options, see **[docs/INSTALLATION.md](docs/INSTALLATION.md)**.

## Quick Start

Create an `InstallPlanApprover` resource:

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: my-approver
  namespace: operators
spec:
  # Enable automatic approval
  autoApprove: true
  
  # Target specific namespaces (empty = all namespaces)
  targetNamespaces:
    - cert-manager
    - gitlab-runner-operator
  
  # Optionally filter by operator names (empty = all operators)
  operatorNames:
    - cert-manager
    - gitlab-runner-operator
```

Apply it:
```bash
kubectl apply -f approver.yaml
```

**Check status:**
```bash
kubectl get installplanapprovers -A
kubectl get installplanapprover my-approver -n operators -o yaml
```

The operator will automatically approve matching InstallPlans in the specified namespaces!

## Configuration

### Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `autoApprove` | bool | Enable/disable automatic approval | `true` |
| `targetNamespaces` | []string | Namespaces to watch (empty = all) | `[]` |
| `operatorNames` | []string | Operator names to approve (empty = all) | `[]` |

### Status

| Field | Type | Description |
|-------|------|-------------|
| `approvedCount` | int32 | Total InstallPlans approved |
| `lastApprovedPlan` | string | Last approved plan (namespace/name) |
| `lastApprovedTime` | metav1.Time | Timestamp of last approval |

For detailed configuration examples, see **[docs/CONFIGURATION.md](docs/CONFIGURATION.md)**.

## How It Works

The operator is **primarily event-driven**:

1. Watches for `InstallPlanApprover` custom resources
2. Watches for `InstallPlan` creation/updates via informers
3. When an InstallPlan is created:
   - Finds the owning Subscription
   - Compares InstallPlan's CSV with Subscription's `startingCSV`
   - Approves **only if versions match exactly**
   - Updates status with approval count and timestamp

**Intelligent requeue strategy:**
- ✅ All approved: No periodic requeue (pure event-driven)
- ⏱️ Non-matching plans found: Requeue after 3 minutes (won't change without Git update)
- 🔄 Otherwise: Requeue after 1 minute (safety net for missed events)

For architecture details, see **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)**.

## GitOps Integration

Works seamlessly with ArgoCD and other GitOps tools. The operator includes:
- Custom ArgoCD health checks for version-pinned operators
- Sync wave support for proper deployment ordering
- Status reporting for GitOps dashboards

See **[docs/GITOPS.md](docs/GITOPS.md)** for detailed integration examples.

## Documentation

- **[Installation Guide](docs/INSTALLATION.md)** - Detailed installation options
- **[Configuration Guide](docs/CONFIGURATION.md)** - Configuration examples and patterns
- **[Architecture](docs/ARCHITECTURE.md)** - How it works, reconciliation logic
- **[GitOps Integration](docs/GITOPS.md)** - ArgoCD/Flux integration
- **[Troubleshooting](docs/TROUBLESHOOTING.md)** - Common issues and solutions
- **[Development Guide](docs/DEVELOPMENT.md)** - Contributing and local development

## Comparison with Alternatives

| Method | Event-Driven | Version Control | Race Conditions | API Load |
|--------|-------------|-----------------|-----------------|----------|
| **InstallPlanApprover Operator** | ✅ Yes | ✅ Yes (CSV match) | ❌ None | ⚡ Minimal |
| Manual `kubectl patch` | ❌ No | ✅ Yes | ❌ None | ⚡ None |
| CronJob polling | ❌ No | ❌ No | ⚠️ Possible | ⚠️ High |

## Development

```bash
# Install CRDs
make install

# Run locally
make run

# Run tests
make test

# Build image
make docker-build IMG=<your-registry>/installplan-approver-operator:latest
```

See **[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)** for detailed development instructions.

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
