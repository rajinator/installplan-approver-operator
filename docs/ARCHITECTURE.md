# Architecture

This document describes the internal architecture and reconciliation logic of the InstallPlan Approver Operator.

## Components

### 1. Controller

The `InstallPlanApproverReconciler` reconciles `InstallPlanApprover` custom resources and processes InstallPlans.

**Key responsibilities:**
- Watch for InstallPlanApprover CR changes
- Process InstallPlans in target namespaces
- Match InstallPlan CSV with Subscription's `startingCSV`
- Approve matching InstallPlans
- Update status with approval metrics

### 2. Informers

Informers provide efficient, event-driven watching of Kubernetes resources:

- **InstallPlanApprover Informer**: Watches for CR create/update/delete events
- **InstallPlan Informer**: Watches for InstallPlan create/update events
- **Subscription Informer**: Watches for Subscription changes (for version matching)

**Benefits:**
- Real-time event notifications
- No polling required
- Minimal API server load

### 3. Listers

Listers provide cached access to resources, reducing API calls:

- List InstallPlans in target namespaces
- List Subscriptions for version matching
- Filter by operator names when configured

## How It Works

### Event Flow

```
┌─────────────────────────────────────────────────────────────────┐
│  1. User creates/updates InstallPlanApprover CR                 │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  2. Controller watches CR via Informer                          │
│     - Triggers immediate reconciliation                         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  3. Reconcile Loop                                              │
│     For each target namespace:                                  │
│       a. List all InstallPlans                                  │
│       b. For each unapproved InstallPlan:                       │
│          - Find owning Subscription                             │
│          - Compare CSV with startingCSV                         │
│          - Approve if exact match                               │
│          - Log if non-matching (skip approval)                  │
│       c. Update status (count, timestamp)                       │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  4. InstallPlan Watch Trigger                                   │
│     - New InstallPlan created → triggers reconciliation         │
│     - Approval happens within milliseconds                      │
└─────────────────────────────────────────────────────────────────┘
```

### Version Matching Logic

**Critical feature:** The operator only approves InstallPlans whose CSV **exactly matches** the Subscription's `startingCSV`.

```go
// Pseudo-code
func shouldApproveInstallPlan(plan InstallPlan, sub Subscription) bool {
    if sub.spec.startingCSV == "" {
        return false  // No version pinning
    }
    
    if plan.spec.clusterServiceVersionNames[0] == sub.spec.startingCSV {
        return true  // ✅ Exact match - approve
    }
    
    return false  // ❌ Non-matching (likely upgrade) - skip
}
```

**Example:**
```yaml
# Subscription in Git
spec:
  startingCSV: prometheus-operator.v0.68.0

# InstallPlan for v0.68.0 → ✅ Approved
# InstallPlan for v0.69.0 → ❌ Not approved (requires Git update)
```

### Reconciliation Strategy

The operator uses an **intelligent requeue strategy** to minimize API load:

#### Event-Driven (Primary Mode)

The operator reacts immediately to events:
- InstallPlanApprover CR create/update/delete
- InstallPlan create/update

**No periodic reconciliation needed** when all plans are approved.

#### Intelligent Requeue (Backup Mode)

After processing, the controller decides whether to requeue:

```go
// Actual code logic
if allApproved {
    return ctrl.Result{}, nil  // ✅ No requeue - pure event-driven
}

if foundNonMatchingPlans {
    // Non-matching plans won't change without Subscription update
    return ctrl.Result{RequeueAfter: 3 * time.Minute}, nil
}

// Safety net for missed events
return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
```

| Scenario | Requeue Delay | Reason |
|----------|---------------|--------|
| All plans approved | None | Event-driven mode, no polling needed |
| Non-matching plans exist | 3 minutes | Won't change without Git update |
| Unapproved plans exist | 1 minute | Safety net for edge cases |

### Reconciliation Triggers

The operator reconciles when:

1. **InstallPlanApprover CR changes**
   - Create: Start watching for InstallPlans
   - Update: Adjust target namespaces/operators
   - Delete: Stop processing

2. **InstallPlan events**
   - Create: New InstallPlan → check and approve
   - Update: Status change → re-evaluate

3. **Scheduled requeue** (if applicable)
   - Safety net for missed events
   - Intelligent delay based on state

### Namespace Handling

The operator supports three namespace targeting modes:

#### 1. All Namespaces
```yaml
spec:
  targetNamespaces: []  # Empty = all
```
- Watches InstallPlans across entire cluster
- Requires cluster-scoped RBAC

#### 2. Specific Namespaces
```yaml
spec:
  targetNamespaces:
    - cert-manager
    - gitlab-runner-operator
```
- Watches only specified namespaces
- More efficient for large clusters

#### 3. Single Namespace
```yaml
spec:
  targetNamespaces:
    - monitoring
```
- Isolated approval for one namespace
- Useful for testing or security boundaries

### Operator Name Filtering

Optionally filter by operator name:

```yaml
spec:
  operatorNames:
    - cert-manager
    - prometheus
```

**How it works:**
1. List all InstallPlans in target namespaces
2. For each plan:
   - Check if `spec.clusterServiceVersionNames[0]` starts with any operator name
   - Skip if no match (when `operatorNames` is non-empty)
3. Proceed with version matching for filtered plans

## RBAC Requirements

The operator requires these permissions:

### Cluster-Scoped

```yaml
- apiGroups: ["operators.coreos.com"]
  resources: ["installplans"]
  verbs: ["get", "list", "watch", "update", "patch"]

- apiGroups: ["operators.coreos.com"]
  resources: ["subscriptions"]
  verbs: ["get", "list", "watch"]

- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch"]
```

### Namespace-Scoped (InstallPlanApprover CR)

```yaml
- apiGroups: ["operators.bapu.cloud"]
  resources: ["installplanapprovers"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

- apiGroups: ["operators.bapu.cloud"]
  resources: ["installplanapprovers/status"]
  verbs: ["get", "update", "patch"]
```

## Status Reporting

The operator updates the CR status after each reconciliation:

```yaml
status:
  approvedCount: 42
  lastApprovedPlan: "monitoring/install-xyz123"
  lastApprovedTime: "2025-10-21T12:34:56Z"
```

**Fields:**
- `approvedCount`: Cumulative count of all approved InstallPlans
- `lastApprovedPlan`: Most recent approval (namespace/name format)
- `lastApprovedTime`: RFC3339 timestamp

**Use cases:**
- Monitoring dashboards
- Audit trails
- GitOps health checks
- Debugging

## Performance Considerations

### API Server Load

The operator minimizes API load through:

1. **Informers with caching**: Watch events without polling
2. **Listers for queries**: Read from cache, not API
3. **Intelligent requeue**: No periodic reconciliation when idle
4. **Namespace filtering**: Only watch relevant namespaces

### Scalability

Tested with:
- ✅ 100+ operators across multiple namespaces
- ✅ InstallPlan approval within milliseconds
- ✅ Minimal CPU/memory footprint

### Resource Usage

**Typical resource consumption:**
- CPU: 1-50m (idle) / 100-200m (active)
- Memory: 25m (idle) - 100Mi

## Error Handling

### Graceful Degradation

- **API errors**: Log and requeue (exponential backoff)
- **Invalid Subscriptions**: Skip and log warning
- **RBAC errors**: Log error, continue processing other plans
- **Transient failures**: Automatic retry via requeue

### Logging

The operator provides structured logging:

```
INFO  Reconciling InstallPlanApprover  namespace=operators name=my-approver
INFO  Processing namespace  namespace=cert-manager
INFO  Approved InstallPlan  namespace=cert-manager name=install-abc123 csv=cert-manager.v1.15.0
WARN  Skipping non-matching InstallPlan  plan-csv=cert-manager.v1.16.0 subscription-csv=cert-manager.v1.15.0
INFO  Status updated  approvedCount=1 lastPlan=cert-manager/install-abc123
```

## Future Enhancements

Potential improvements:

- [ ] Label-based filtering
- [ ] Approval policies (e.g., only approve within business hours)
- [ ] Webhook validation for InstallPlanApprover CRs
- [ ] Metrics endpoint (Prometheus)
- [ ] Multi-cluster support
- [ ] Approval notifications (Slack, webhook)

