# Configuration Guide

Complete configuration guide for InstallPlanApprover resources.

## Basic Configuration

### Minimal Configuration

Approve all InstallPlans in all namespaces:

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: cluster-wide-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces: []  # Empty = all namespaces
```

### Single Namespace

Approve InstallPlans in one namespace:

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: single-namespace-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces:
    - cert-manager
```

### Multiple Namespaces

Approve InstallPlans across multiple namespaces:

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: multi-namespace-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces:
    - openshift-gitops-operator
    - cert-manager
    - gitlab-runner-operator
```

## Advanced Configuration

### Operator Name Filtering

Approve only specific operators:

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: filtered-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces:
    - monitoring
  operatorNames:
    - prometheus
    - grafana-operator
    - alertmanager
```

**How it works:**
- Only approves InstallPlans for operators starting with these names
- Empty `operatorNames` means approve all operators

### Disable Auto-Approval

Create the resource but don't auto-approve (for testing):

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: manual-approver
  namespace: operators
spec:
  autoApprove: false  # Operator won't approve
  targetNamespaces:
    - test-operators
```

**Use cases:**
- Testing the operator without affecting InstallPlans
- Temporarily disabling approval for maintenance

## Field Reference

### Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `autoApprove` | bool | No | `true` | Enable/disable automatic approval |
| `targetNamespaces` | []string | No | `[]` (all) | Namespaces to watch for InstallPlans |
| `operatorNames` | []string | No | `[]` (all) | Operator names to approve (prefix match) |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `approvedCount` | int32 | Cumulative count of approved InstallPlans |
| `lastApprovedPlan` | string | Last approved plan (format: `namespace/name`) |
| `lastApprovedTime` | metav1.Time | RFC3339 timestamp of last approval |

## Common Patterns

### Pattern 1: Development Cluster

Approve everything (least restrictive):

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: dev-cluster-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces: []  # All namespaces
  operatorNames: []     # All operators
```

### Pattern 2: Production Cluster

Approve only specific operators in specific namespaces:

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: prod-cluster-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces:
    - production-monitoring
    - production-logging
  operatorNames:
    - prometheus
    - grafana-operator
    - elasticsearch-operator
```

### Pattern 3: Team-Based

Different approvers for different teams:

```yaml
---
# Platform team operators
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: platform-team-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces:
    - openshift-gitops-operator
    - openshift-pipelines-operator
---
# Application team operators
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: app-team-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces:
    - cert-manager
    - external-secrets
```

### Pattern 4: GitOps Workflow

Centralized approval for all GitOps-managed operators:

```yaml
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: gitops-managed-approver
  namespace: operators
  labels:
    managed-by: argocd
spec:
  autoApprove: true
  targetNamespaces:
    - openshift-gitops-operator
    - cert-manager
    - gitlab-runner-operator
  operatorNames:
    - openshift-gitops-operator
    - cert-manager
    - gitlab-runner-operator
```

## Status Monitoring

### View Status

```bash
# List all approvers
kubectl get installplanapprovers -A

# Get detailed status
kubectl get installplanapprover my-approver -n operators -o yaml
```

### Example Status

```yaml
status:
  approvedCount: 42
  lastApprovedPlan: "monitoring/install-xyz123"
  lastApprovedTime: "2025-10-21T12:34:56Z"
```

### Query Status with JSONPath

```bash
# Get approval count
kubectl get installplanapprover my-approver -n operators -o jsonpath='{.status.approvedCount}'

# Get last approved plan
kubectl get installplanapprover my-approver -n operators -o jsonpath='{.status.lastApprovedPlan}'

# Get last approval time
kubectl get installplanapprover my-approver -n operators -o jsonpath='{.status.lastApprovedTime}'
```

## Multiple Approvers

You can create multiple InstallPlanApprover resources:

```yaml
---
# Approver 1: Monitoring namespace
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: monitoring-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces:
    - monitoring
---
# Approver 2: Security namespace
apiVersion: operators.bapu.cloud/v1alpha1
kind: InstallPlanApprover
metadata:
  name: security-approver
  namespace: operators
spec:
  autoApprove: true
  targetNamespaces:
    - security
```

**Behavior:**
- Each approver operates independently
- No conflicts - InstallPlans are approved by the first matching approver
- Useful for different teams/policies

## Best Practices

### 1. Use Specific Namespaces in Production

❌ **Don't:**
```yaml
targetNamespaces: []  # Too broad for production
```

✅ **Do:**
```yaml
targetNamespaces:
  - prod-monitoring
  - prod-logging
```

### 2. Filter by Operator Names

When possible, specify operator names:

```yaml
operatorNames:
  - prometheus
  - grafana-operator
```

### 3. Label Your Resources

Add labels for organization:

```yaml
metadata:
  name: my-approver
  labels:
    environment: production
    team: platform
    managed-by: argocd
```

### 4. Use Version Pinning

Always use `startingCSV` in Subscriptions:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: prometheus-operator
spec:
  installPlanApproval: Manual
  startingCSV: prometheus-operator.v0.67.0  # ← Always specify
```

### 5. Monitor Status

Set up alerts on status fields:

```bash
# Example: Alert if no approvals in last hour
kubectl get installplanapprovers -A -o json | \
  jq '.items[] | select(.status.lastApprovedTime < (now - 3600))'
```

## Troubleshooting Configuration

### InstallPlans Not Being Approved

**Check 1: Namespace targeting**
```bash
kubectl get installplanapprover my-approver -n operators -o jsonpath='{.spec.targetNamespaces}'
```

**Check 2: Operator name filtering**
```bash
kubectl get installplanapprover my-approver -n operators -o jsonpath='{.spec.operatorNames}'
```

**Check 3: Auto-approve enabled**
```bash
kubectl get installplanapprover my-approver -n operators -o jsonpath='{.spec.autoApprove}'
```

### Version Mismatch

If InstallPlans aren't being approved, check CSV matching:

```bash
# Check Subscription's startingCSV
kubectl get subscription <name> -n <namespace> -o jsonpath='{.spec.startingCSV}'

# Check InstallPlan's CSV
kubectl get installplan <name> -n <namespace> -o jsonpath='{.spec.clusterServiceVersionNames[0]}'

# These must match exactly
```

For more troubleshooting, see [TROUBLESHOOTING.md](TROUBLESHOOTING.md).

