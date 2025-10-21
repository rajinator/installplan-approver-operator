# Troubleshooting Guide

Common issues and solutions for the InstallPlan Approver Operator.

## InstallPlans Not Being Approved

### Check 1: Operator Running?

```bash
kubectl get pods -n iplan-approver-system

# Expected output:
# NAME                                                READY   STATUS    RESTARTS   AGE
# iplan-approver-controller-manager-xxxxxxxxxx-xxxxx   1/1     Running   0          10m
```

**If pod is not running:**
```bash
kubectl describe pod -n iplan-approver-system <pod-name>
kubectl logs -n iplan-approver-system <pod-name>
```

### Check 2: InstallPlanApprover Created?

```bash
kubectl get installplanapprovers -A

# Should show your approver resources
```

**If not found:**
```bash
# Create one
kubectl apply -f config/samples/operators_v1alpha1_installplanapprover.yaml
```

### Check 3: Namespace Targeted?

```bash
kubectl get installplanapprover <name> -n <namespace> -o yaml
```

Check `spec.targetNamespaces` includes the InstallPlan's namespace.

**Example:**
```yaml
spec:
  targetNamespaces:
    - cert-manager  # Must include InstallPlan's namespace
```

### Check 4: Version Matches?

**Critical check:** InstallPlan CSV must match Subscription's `startingCSV`.

```bash
# Check Subscription's startingCSV
kubectl get subscription <name> -n <namespace> -o jsonpath='{.spec.startingCSV}'

# Check InstallPlan's CSV
kubectl get installplan <name> -n <namespace> -o jsonpath='{.spec.clusterServiceVersionNames[0]}'

# These must match exactly
```

**If they don't match:**
- The operator is working correctly
- Update `startingCSV` in your Subscription (in Git) to approve

### Check 5: Operator Logs

```bash
kubectl logs -n iplan-approver-system -l control-plane=controller-manager --tail=100
```

**Look for:**
```
INFO  Approved InstallPlan  namespace=cert-manager name=install-abc123 csv=cert-manager.v1.15.0
WARN  Skipping non-matching InstallPlan  plan-csv=cert-manager.v1.16.0 subscription-csv=cert-manager.v1.15.0
```

### Check 6: AutoApprove Enabled?

```bash
kubectl get installplanapprover <name> -n <namespace> -o jsonpath='{.spec.autoApprove}'
```

Should return `true`.

## InstallPlan Approved But Operator Not Installing

This is an **OLM issue**, not the InstallPlanApprover operator.

### Check InstallPlan Status

```bash
kubectl get installplan <name> -n <namespace> -o yaml
```

Look for:
```yaml
status:
  phase: Complete  # Should be Complete
```

### Check CSV Status

```bash
kubectl get csv -n <namespace>
```

**Expected states:**
- `Succeeded`: Operator installed successfully
- `Installing`: Operator installation in progress
- `Failed`: Installation failed (check CSV description)

### Check CSV Details

```bash
kubectl describe csv <csv-name> -n <namespace>
```

### Check Operator Pod Logs

```bash
kubectl logs -n <namespace> -l app=<operator-name>
```

## RBAC Permission Errors

### Symptom

Operator logs show:
```
ERROR  installplans.operators.coreos.com is forbidden: User "system:serviceaccount:iplan-approver-system:iplan-approver-controller-manager" cannot list resource "installplans"
```

### Solution

Check ClusterRoleBinding:

```bash
kubectl get clusterrolebinding iplan-approver-manager-rolebinding -o yaml
```

Verify `subjects[0].namespace` matches operator namespace:

```yaml
subjects:
- kind: ServiceAccount
  name: iplan-approver-controller-manager
  namespace: iplan-approver-system  # Must match operator namespace
```

**Fix if wrong:**
```bash
kubectl patch clusterrolebinding iplan-approver-manager-rolebinding --type=json \
  -p='[{"op": "replace", "path": "/subjects/0/namespace", "value": "iplan-approver-system"}]'
```

## Image Pull Errors

### Symptom

```bash
kubectl describe pod -n iplan-approver-system

# Shows:
# Failed to pull image "controller:latest"
```

### Cause

Placeholder image not replaced with actual image.

### Solution

Re-deploy with correct image:

```bash
kubectl apply -k 'github.com/rajinator/installplan-approver-operator/config/default?ref=v0.1.0'
```

Or with make:

```bash
make deploy IMG=ghcr.io/rajinator/installplan-approver-operator:v0.1.0
```

## CRD Issues

### CRD Not Found

```bash
kubectl get crd installplanapprovers.operators.bapu.cloud

# Error from server (NotFound): customresourcedefinitions.apiextensions.k8s.io "installplanapprovers.operators.bapu.cloud" not found
```

**Solution:**
```bash
make install
# Or
kubectl apply -k github.com/rajinator/installplan-approver-operator/config/crd
```

### CRD Version Mismatch

After upgrading, CRD may be outdated.

**Solution:**
```bash
# Reinstall CRDs
make uninstall
make install

# Or
kubectl replace -f config/crd/bases/operators.bapu.cloud_installplanapprovers.yaml
```

## OLM Not Installed

### Symptom

```bash
kubectl get crd installplans.operators.coreos.com

# Error from server (NotFound)
```

### Solution

Install OLM:

```bash
operator-sdk olm install
```

Or follow OLM installation guide: https://olm.operatorframework.io/docs/getting-started/

## Multiple InstallPlans Created

### Symptom

Multiple InstallPlans exist for the same operator, some approved, some not.

### Cause

This is **normal OLM behavior**:
1. Initial InstallPlan created → approved by operator
2. Operator installs
3. New version available → new InstallPlan created
4. If version doesn't match `startingCSV` → not approved (correct!)

### Solution

**This is working as designed:**
- Old InstallPlan: Approved (matched `startingCSV`)
- New InstallPlan: Not approved (upgrade requires Git update)

To approve upgrade:
1. Update `startingCSV` in Subscription (in Git)
2. ArgoCD/Flux syncs the change
3. New InstallPlan automatically approved

## Operator Logs Show Warnings

### Warning: Non-Matching InstallPlan

```
WARN  Skipping non-matching InstallPlan  plan-csv=cert-manager.v1.16.0 subscription-csv=cert-manager.v1.15.0
```

**This is expected behavior:**
- Operator found an upgrade InstallPlan
- Version doesn't match pinned version
- Operator correctly skips approval

**No action needed** unless you want to upgrade:
- Update `startingCSV` in Git to v1.16.0

### Warning: No Subscription Found

```
WARN  No Subscription found for InstallPlan  installplan=cert-manager/install-abc123
```

**Possible causes:**
1. InstallPlan has no owner reference
2. Subscription deleted
3. Subscription in different namespace (shouldn't happen)

**Solution:**
Check if Subscription exists:
```bash
kubectl get subscriptions -n cert-manager
```

## ArgoCD Application Stuck in Progressing

### Symptom

ArgoCD shows Subscription stuck in `Progressing` phase despite operator working correctly.

### Cause

Default ArgoCD health checks don't understand version pinning with pending upgrade InstallPlans.

### Solution

Add custom health checks to ArgoCD. See [docs/GITOPS.md](GITOPS.md) for details.

## Performance Issues

### High API Server Load

**Check operator logs for frequent reconciliations:**
```bash
kubectl logs -n iplan-approver-system -l control-plane=controller-manager | grep "Reconciling"
```

**Solution:**
- Use namespace filtering to reduce scope
- Check for misconfigured approver causing frequent requeues

### Operator Using Too Much Memory

**Check resource usage:**
```bash
kubectl top pod -n iplan-approver-system
```

**If memory usage is high (>500Mi):**
- May indicate large number of InstallPlans
- Check for resource leaks (file issue on GitHub)

**Workaround:**
```bash
# Restart operator
kubectl rollout restart deployment/iplan-approver-controller-manager -n iplan-approver-system
```

## Debugging Tips

### Enable Verbose Logging

Edit operator deployment to increase log level:

```bash
kubectl edit deployment iplan-approver-controller-manager -n iplan-approver-system
```

Add environment variable:
```yaml
env:
- name: LOG_LEVEL
  value: "debug"
```

### Watch Events

```bash
# Watch all events in operator namespace
kubectl get events -n iplan-approver-system --watch

# Watch InstallPlan events
kubectl get events -n <target-namespace> --field-selector involvedObject.kind=InstallPlan --watch
```

### Trace Reconciliation

```bash
# Follow operator logs while creating approver
kubectl logs -n iplan-approver-system -l control-plane=controller-manager -f

# In another terminal, create approver
kubectl apply -f my-approver.yaml
```

### Check Informer Cache

Operator uses informers with cache. Wait a few seconds after creating resources for cache to sync.

## Getting Help

If you're still stuck:

1. **Check GitHub Issues**: https://github.com/rajinator/installplan-approver-operator/issues
2. **File a Bug Report**:
   - Include operator logs
   - Include InstallPlanApprover CR
   - Include relevant InstallPlan and Subscription manifests
   - Include Kubernetes/OpenShift version
3. **Check OLM Documentation**: https://olm.operatorframework.io/

## Common Misconfigurations

### ❌ Empty targetNamespaces AND Empty operatorNames

```yaml
spec:
  targetNamespaces: []
  operatorNames: []
```

**Impact:** Approves everything (may be too broad for production)

### ❌ Typo in Operator Name

```yaml
spec:
  operatorNames:
    - cert-manger  # Typo! Should be cert-manager
```

**Impact:** Won't approve cert-manager InstallPlans

### ❌ Wrong Namespace in targetNamespaces

```yaml
spec:
  targetNamespaces:
    - certificate-manager  # Wrong! Should be cert-manager
```

**Impact:** Won't watch correct namespace

### ❌ Missing startingCSV in Subscription

```yaml
# Subscription
spec:
  installPlanApproval: Manual
  # startingCSV missing!
```

**Impact:** Operator won't approve (no version to match against)

