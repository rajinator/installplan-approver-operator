# Implementation Details

## Overview

The InstallPlan Approver Operator is built using the Operator SDK with a focus on efficiency through the use of informers and listers. This document describes the implementation details.

## Architecture

### Components

1. **API Types** (`api/v1alpha1/installplanapprover_types.go`)
   - Defines the `InstallPlanApprover` CRD
   - Spec fields: `targetNamespaces`, `autoApprove`, `operatorNames`
   - Status fields: `approvedCount`, `lastApprovedPlan`, `lastApprovedTime`

2. **Controller** (`internal/controller/installplanapprover_controller.go`)
   - Main reconciliation logic
   - Watches InstallPlanApprover and InstallPlan resources
   - Approves unapproved InstallPlans based on configuration

### Key Design Decisions

#### 1. Use of Unstructured Objects

The operator uses `unstructured.Unstructured` to work with InstallPlan resources without importing the OLM API dependency. This makes the operator:

- Lightweight (no extra dependencies)
- Compatible with any OLM version
- Easy to build and deploy

```go
installPlanGVK := schema.GroupVersionKind{
    Group:   "operators.coreos.com",
    Version: "v1alpha1",
    Kind:    "InstallPlan",
}

installPlanList := &unstructured.UnstructuredList{}
installPlanList.SetGroupVersionKind(installPlanGVK)
```

#### 2. Informers and Listers

The operator leverages controller-runtime's built-in informer and lister mechanisms:

- **Informers**: Automatically set up when using `ctrl.NewControllerManagedBy(mgr).Watches()`
- **Listers**: Accessed through the client's cache (`r.List()` uses cached data)
- **Event-driven**: Reconciles immediately when resources change
- **Periodic Reconciliation**: Also reconciles every 30 seconds to catch any missed events

```go
// SetupWithManager configures watches
return ctrl.NewControllerManagedBy(mgr).
    For(&operatorsv1alpha1.InstallPlanApprover{}).
    Watches(
        installPlanUnstructured,
        handler.EnqueueRequestsFromMapFunc(r.findApproversForInstallPlan),
    ).
    Complete(r)
```

#### 3. Efficient Resource Watching

**Primary Watch**: InstallPlanApprover resources
- Triggers reconciliation when approvers are created/updated/deleted

**Secondary Watch**: InstallPlan resources
- Uses `EnqueueRequestsFromMapFunc` to map InstallPlan events to all InstallPlanApprover objects
- Ensures immediate response to new InstallPlans

#### 4. Graceful Degradation

The operator handles cases where OLM is not installed:

```go
if err := r.List(ctx, installPlanList, listOptions); err != nil {
    // If CRD doesn't exist, just log and continue
    if errors.IsNotFound(err) || runtime.IsNotRegisteredError(err) {
        logger.V(1).Info("InstallPlans CRD not found in cluster, skipping", "namespace", namespace)
        return 0, "", nil, nil
    }
    return 0, "", nil, err
}
```

## Reconciliation Flow

```
1. Reconcile triggered (by InstallPlanApprover or InstallPlan event)
   ↓
2. Fetch InstallPlanApprover resource
   ↓
3. Check if autoApprove is enabled
   ↓
4. Determine target namespaces (specified or all)
   ↓
5. For each namespace:
   a. List InstallPlans (from cache via informer)
   b. Filter unapproved InstallPlans
   c. Check operator name filter (if specified)
   d. Approve InstallPlans (set spec.approved: true)
   e. Track approvals
   ↓
6. Update InstallPlanApprover status
   ↓
7. Requeue after 30 seconds
```

## Performance Characteristics

### API Server Load

- **Minimal API Calls**: Uses informers to cache resources locally
- **List Operations**: Served from cache (no API server hit)
- **Update Operations**: Only when approving InstallPlans
- **Watch Efficiency**: Single watch per resource type (shared across reconciliations)

### Memory Usage

- Caches all InstallPlanApprover resources in the cluster
- Caches InstallPlan resources across watched namespaces
- Typical memory footprint: ~50-100MB base + ~1KB per InstallPlan

### Reconciliation Performance

- **Event-driven**: Immediate response (< 100ms) to new InstallPlans
- **Periodic**: 30-second reconciliation loop as safety net
- **Batch Processing**: Processes all InstallPlans in a namespace together

## RBAC Requirements

The operator requires cluster-wide permissions:

```yaml
- apiGroups: [""]
  resources: [namespaces]
  verbs: [get, list, watch]

- apiGroups: [operators.bapu.cloud]
  resources: [installplanapprovers]
  verbs: [get, list, watch, create, update, patch, delete]

- apiGroups: [operators.bapu.cloud]
  resources: [installplanapprovers/status]
  verbs: [get, update, patch]

- apiGroups: [operators.coreos.com]
  resources: [installplans]
  verbs: [get, list, watch, update, patch]
```

## Filtering Logic

### Namespace Filtering

```go
// Empty targetNamespaces = all namespaces
if len(namespaces) == 0 {
    namespaceList := &corev1.NamespaceList{}
    if err := r.List(ctx, namespaceList); err != nil {
        return ctrl.Result{}, err
    }
    for _, ns := range namespaceList.Items {
        namespaces = append(namespaces, ns.Name)
    }
}
```

### Operator Name Filtering

```go
// Check if operator name is in the allowed list (if specified)
if len(operatorNames) > 0 {
    clusterServiceVersionNames, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "clusterServiceVersionNames")
    if !r.isOperatorAllowed(clusterServiceVersionNames, operatorNames) {
        logger.V(1).Info("Operator not in allowed list, skipping", "installplan", item.GetName())
        continue
    }
}
```

Matching logic:
- Exact match: `csvName == allowedName`
- Prefix match: `csvName` starts with `allowedName`

## Error Handling

### Transient Errors

- Network errors: Requeued automatically by controller-runtime
- API server unavailable: Reconciliation retried with exponential backoff

### Permanent Errors

- CRD not found: Logged and skipped (graceful degradation)
- Invalid resource: Logged and skipped (prevents crash loops)

### Status Updates

- Updates only if successful approvals occurred
- Failures logged but don't prevent other approvals
- Status update failures logged but don't block reconciliation

## Testing Strategy

### Unit Tests

Located in `internal/controller/installplanapprover_controller_test.go`:

- Test reconciliation with valid InstallPlanApprover
- Test filtering by namespace
- Test filtering by operator name
- Test graceful handling of missing CRDs

### Integration Tests

Use the provided test script (`test-operator.sh`):

```bash
# Quick integration test
./test-operator.sh full-test
./test-operator.sh run

# In another terminal
./test-operator.sh create-test
./test-operator.sh watch-ns test-operators
```

### Manual Testing

See [QUICKSTART.md](QUICKSTART.md) for step-by-step manual testing procedures.

## Future Enhancements

### Potential Improvements

1. **Approval Policies**
   - Allow/deny lists with regex patterns
   - Time-based approval windows
   - Approval webhooks for external validation

2. **Advanced Filtering**
   - Filter by InstallPlan phase
   - Filter by CSV version (semver ranges)
   - Label selectors for namespaces

3. **Observability**
   - Prometheus metrics (approvals, failures, latency)
   - Events on approvals
   - Detailed status conditions

4. **Multi-tenancy**
   - Namespace-scoped InstallPlanApprovers
   - RBAC-based approval delegation
   - Quota limits on approvals

5. **Safety Features**
   - Dry-run mode
   - Approval rate limiting
   - Rollback support

## Troubleshooting Implementation Issues

### Controller Not Reconciling

Check:
1. Controller logs for errors
2. Manager is running and healthy
3. RBAC permissions are correct
4. CRDs are installed

### InstallPlans Not Being Approved

Check:
1. InstallPlanApprover `autoApprove` is `true`
2. Namespace filtering is correct
3. Operator name filtering is not too restrictive
4. InstallPlan CRD exists in cluster

### High Memory Usage

Possible causes:
1. Too many namespaces being watched
2. Large number of InstallPlans in cluster
3. Memory leak (report a bug!)

Solutions:
- Limit `targetNamespaces` to specific namespaces
- Use operator name filtering to reduce scope
- Increase memory limits in deployment

### Performance Issues

Check:
1. Reconciliation frequency (30s default)
2. Number of InstallPlans being processed
3. API server response times

Optimizations:
- Reduce reconciliation frequency
- Use more specific namespace targeting
- Scale operator replicas (with leader election)

## Code Organization

```
internal/controller/
  └── installplanapprover_controller.go
      ├── InstallPlanApproverReconciler (struct)
      ├── Reconcile() (main reconciliation)
      ├── processNamespace() (namespace-level processing)
      ├── isOperatorAllowed() (filtering logic)
      ├── SetupWithManager() (watch setup)
      └── findApproversForInstallPlan() (event mapping)
```

## Dependencies

### Direct Dependencies

- `sigs.k8s.io/controller-runtime`: Core operator framework
- `k8s.io/api`: Kubernetes core API types
- `k8s.io/apimachinery`: Kubernetes machinery (schema, runtime, etc.)

### No OLM Dependency

The operator intentionally does NOT import OLM API types:
- Reduces binary size
- Avoids version compatibility issues
- Uses dynamic (unstructured) clients instead

## Build and Deployment

### Local Build

```bash
make build
```

Binary: `bin/manager`

### Container Image

```bash
make docker-build IMG=<your-registry>/installplanapprover-operator:latest
make docker-push IMG=<your-registry>/installplanapprover-operator:latest
```

### Deployment

```bash
make deploy IMG=<your-registry>/installplanapprover-operator:latest
```

Deploys to: `installplan-approver-operator-system` namespace

## References

- [Operator SDK Documentation](https://sdk.operatorframework.io/)
- [Controller Runtime Documentation](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [OLM Documentation](https://olm.operatorframework.io/)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)

