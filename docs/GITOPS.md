# GitOps Integration

Complete guide for integrating the InstallPlan Approver Operator with GitOps tools.

## ArgoCD Integration

### Overview

The InstallPlan Approver Operator is designed for GitOps workflows:

✅ **Declarative Configuration**: Define operator versions in Git  
✅ **Automated Approval**: No manual intervention required  
✅ **Version Control**: Git is the single source of truth  
✅ **Audit Trail**: All changes tracked via Git history  

### Deployment Architecture

```
┌─────────────────────────────────────────────────────────┐
│  App-of-Apps (dev-cluster-apps)                         │
│  Manages all applications via ArgoCD                    │
└────────────────────┬────────────────────────────────────┘
                     │
        ┌────────────┼────────────┬─────────────┐
        │            │            │             │
    ┌───▼───┐   ┌───▼───┐   ┌───▼───┐    ┌───▼───┐
    │ IPA   │   │GitOps │   │ Cert  │    │GitLab │
    │Operator│   │Operator│   │Manager│    │Runner │
    │       │   │       │   │       │    │       │
    │Wave 1 │   │Wave 3 │   │Wave 5 │    │Wave 5 │
    └───┬───┘   └───────┘   └───┬───┘    └───┬───┘
        │                        │            │
        │   ┌────────────────────┴────────────┘
        │   │
        ▼   ▼
    ┌───────────────────────────────────────┐
    │  InstallPlan Approver watches and     │
    │  auto-approves version-matched plans  │
    └───────────────────────────────────────┘
```

### Step 1: Deploy the Operator

Create an ArgoCD Application for the operator:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: installplan-approver-operator
  namespace: openshift-gitops
  labels:
    app-type: operator
    argoproj.io/sync-wave: "1"  # Deploy early
spec:
  project: default
  
  source:
    repoURL: https://github.com/rajinator/installplan-approver-operator
    targetRevision: v0.1.0
    path: config/default
  
  destination:
    name: in-cluster
    namespace: iplan-approver-system
  
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

**Key points:**
- `sync-wave: "1"`: Deploy before other operators
- `automated`: Auto-sync on Git changes
- `CreateNamespace=true`: Create namespace if needed

### Step 2: Create InstallPlanApprover CR

Deploy the CR to configure approval policy:

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
  operatorNames:
    - openshift-gitops-operator
    - cert-manager
    - gitlab-runner-operator
```

**Manage via ArgoCD:**
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: installplan-approver-cr
  namespace: openshift-gitops
  labels:
    argoproj.io/sync-wave: "2"  # After operator
spec:
  project: default
  source:
    repoURL: https://github.com/myorg/my-gitops-repo
    targetRevision: HEAD
    path: operators/installplan-approver-cr
  destination:
    name: in-cluster
    namespace: operators
```

### Step 3: Deploy Managed Operators

Configure other operators to use centralized approval:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cert-manager-operator
  namespace: openshift-gitops
  labels:
    app-type: operator
    approval-mode: centralized  # ← Indicates centralized approval
    argoproj.io/sync-wave: "5"  # ← Deploy after approver
spec:
  project: default
  
  source:
    repoURL: https://github.com/myorg/k8s-apps-repo
    targetRevision: HEAD
    path: operators/cert-manager
  
  destination:
    name: in-cluster
    namespace: cert-manager
  
  syncPolicy:
    automated:
      prune: false
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

**Operator Subscription (in Git):**
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  channel: stable
  name: cert-manager
  source: operatorhubio-catalog
  sourceNamespace: olm
  installPlanApproval: Manual  # ← Required for version control
  startingCSV: cert-manager.v1.15.0  # ← Version pinned in Git
```

### Sync Waves Strategy

| Wave | Components | Purpose |
|------|-----------|---------|
| 0 | OpenShift GitOps custom config | Configure ArgoCD first |
| 1 | InstallPlan Approver Operator | Enable auto-approval |
| 2 | InstallPlanApprover CRs | Configure approval policy |
| 3 | OpenShift GitOps self-management | Let ArgoCD manage itself |
| 5+ | Application operators | Deploy with auto-approval |

**Example labels:**
```yaml
metadata:
  labels:
    argoproj.io/sync-wave: "1"  # Operator deploys first
---
metadata:
  labels:
    argoproj.io/sync-wave: "5"  # Apps deploy after approval is ready
```

## Custom Health Checks

### Why Custom Health Checks?

Default ArgoCD health checks don't understand version pinning:

❌ **Without custom checks:**
- Subscription with pending upgrade InstallPlan → `Progressing` (forever)
- Unapproved InstallPlan → `Progressing` (confusing)

✅ **With custom checks:**
- Subscription with matching installed version → `Healthy`
- Unapproved upgrade InstallPlan → `Suspended` (intentional)

### InstallPlan Health Check

Add to your ArgoCD instance:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: openshift-gitops
  namespace: openshift-gitops
spec:
  resourceHealthChecks:
    - group: operators.coreos.com
      kind: InstallPlan
      check: |
        hs = {}
        
        -- Unapproved InstallPlans are "Suspended" (intentionally paused)
        if obj.spec.approved == false then
          hs.status = "Suspended"
          hs.message = "InstallPlan not approved (waiting for version match)"
          return hs
        end
        
        -- Check phase for approved plans
        if obj.status ~= nil and obj.status.phase ~= nil then
          if obj.status.phase == "Complete" then
            hs.status = "Healthy"
            hs.message = "InstallPlan completed"
            return hs
          end
          
          if obj.status.phase == "Failed" then
            hs.status = "Degraded"
            hs.message = "InstallPlan failed: " .. (obj.status.conditions[1].message or "Unknown error")
            return hs
          end
          
          if obj.status.phase == "Installing" then
            hs.status = "Progressing"
            hs.message = "Installing"
            return hs
          end
        end
        
        hs.status = "Progressing"
        hs.message = "Waiting for InstallPlan to complete"
        return hs
```

### Subscription Health Check

```yaml
    - group: operators.coreos.com
      kind: Subscription
      check: |
        hs = {}
        
        if obj.status ~= nil then
          -- Check if installed version matches desired version (version pinning)
          if obj.status.installedCSV ~= nil and obj.spec.startingCSV ~= nil then
            if obj.status.installedCSV == obj.spec.startingCSV then
              hs.status = "Healthy"
              hs.message = "Installed: " .. obj.status.installedCSV .. " (matches startingCSV)"
              return hs
            else
              hs.status = "Progressing"
              hs.message = "Installing " .. obj.spec.startingCSV .. " (current: " .. obj.status.installedCSV .. ")"
              return hs
            end
          end
          
          -- No version pinning (startingCSV not set)
          if obj.status.installedCSV ~= nil then
            if obj.status.state == "AtLatestKnown" then
              hs.status = "Healthy"
              hs.message = "At latest known: " .. obj.status.installedCSV
              return hs
            end
          end
          
          -- Check for errors
          if obj.status.conditions ~= nil then
            for i, condition in ipairs(obj.status.conditions) do
              if condition.type == "ResolutionFailed" and condition.status == "True" then
                hs.status = "Degraded"
                hs.message = condition.message
                return hs
              end
            end
          end
        end
        
        hs.status = "Progressing"
        hs.message = "Waiting for operator installation"
        return hs
```

**Apply health checks:**
```bash
kubectl apply -f argocd-health-checks.yaml
```

## Version Upgrade Workflow

### GitOps Upgrade Process

1. **Developer updates Git:**
   ```yaml
   # In Git: operators/prometheus-subscription.yaml
   spec:
     startingCSV: prometheus-operator.v0.68.0  # Updated from v0.67.0
   ```

2. **ArgoCD syncs change:**
   - Detects diff in Git
   - Updates Subscription on cluster

3. **OLM creates new InstallPlan:**
   - Subscription detects CSV change
   - Creates InstallPlan for v0.68.0

4. **Operator auto-approves:**
   - Receives InstallPlan event
   - Compares v0.68.0 with Subscription's `startingCSV`
   - Exact match → approves immediately

5. **OLM installs upgrade:**
   - InstallPlan approved
   - Operator upgraded to v0.68.0

6. **ArgoCD shows Healthy:**
   - Custom health check verifies installedCSV == startingCSV
   - Application marked as Healthy

### GitOps Approval Flow

```
┌──────────────────────────────────────────────────────────┐
│  1. Update startingCSV in Git                            │
│     prometheus-operator.v0.67.0 → v0.68.0                │
└────────────────────┬─────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────┐
│  2. ArgoCD syncs Subscription                            │
│     Detects diff and applies change                      │
└────────────────────┬─────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────┐
│  3. OLM creates InstallPlan (unapproved)                 │
│     CSV: prometheus-operator.v0.68.0                     │
└────────────────────┬─────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────┐
│  4. InstallPlan Approver watches event                   │
│     Compares v0.68.0 (plan) == v0.68.0 (subscription)    │
│     ✅ Match → Approves immediately                      │
└────────────────────┬─────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────┐
│  5. OLM installs operator                                │
│     Updates CSV, pods, etc.                              │
└────────────────────┬─────────────────────────────────────┘
                     │
                     ▼
┌──────────────────────────────────────────────────────────┐
│  6. ArgoCD shows Healthy                                 │
│     Custom health check: installedCSV == startingCSV     │
└──────────────────────────────────────────────────────────┘
```

## App-of-Apps Pattern

### Structure

```yaml
# bootstrap/my-cluster.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-cluster-apps
  namespace: openshift-gitops
spec:
  project: default
  source:
    repoURL: https://github.com/myorg/gitops-repo
    targetRevision: HEAD
    path: apps/my-cluster
  destination:
    name: in-cluster
    namespace: openshift-gitops
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

```yaml
# apps/my-cluster/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

components:
  - ../components/installplan-approver-operator     # Wave 1
  - ../components/openshift-gitops-custom           # Wave 0
  - ../components/cert-manager-operator             # Wave 5
  - ../components/gitlab-runner-operator            # Wave 5
```

## Monitoring and Observability

### ArgoCD Application Status

```bash
# List all applications
argocd app list

# Get application details
argocd app get installplan-approver-operator

# Watch sync status
argocd app watch installplan-approver-operator
```

### InstallPlanApprover Status

```bash
# Check CR status
kubectl get installplanapprovers -A

# Detailed status
kubectl get installplanapprover multi-namespace-approver -n operators -o yaml
```

**Status fields:**
```yaml
status:
  approvedCount: 42
  lastApprovedPlan: "monitoring/install-xyz123"
  lastApprovedTime: "2025-10-21T12:34:56Z"
```

### Operator Logs

```bash
# Follow operator logs
kubectl logs -n iplan-approver-system -l control-plane=controller-manager -f
```

## Troubleshooting GitOps Issues

### Application Stuck in Progressing

**Symptom:** ArgoCD Application shows `Progressing` indefinitely.

**Check:**
1. Custom health checks installed?
2. InstallPlan approved?
3. Subscription installed CSV matches starting CSV?

```bash
# Check Subscription status
kubectl get subscription cert-manager -n cert-manager -o yaml

# Check InstallPlans
kubectl get installplans -n cert-manager

# Check operator logs
kubectl logs -n iplan-approver-system -l control-plane=controller-manager --tail=50
```

### Sync Fails

**Symptom:** ArgoCD sync fails with error.

**Common causes:**
1. CRD not installed (InstallPlanApprover)
2. Namespace doesn't exist
3. RBAC issues

**Solution:**
```bash
# Check sync status
argocd app get my-app --show-operation

# Force sync
argocd app sync my-app --force
```

## Best Practices

### 1. Use Sync Waves

Deploy InstallPlan Approver before other operators:

```yaml
metadata:
  labels:
    argoproj.io/sync-wave: "1"  # Approver
---
metadata:
  labels:
    argoproj.io/sync-wave: "5"  # Other operators
```

### 2. Add Custom Health Checks

Always configure custom health checks for InstallPlans and Subscriptions.

### 3. Enable Auto-Sync

```yaml
syncPolicy:
  automated:
    prune: false  # Don't auto-delete (safer)
    selfHeal: true  # Auto-sync on drift
```

### 4. Label Resources

```yaml
metadata:
  labels:
    app-type: operator
    approval-mode: centralized
    managed-by: argocd
```

### 5. Centralize Approval

Use one InstallPlan Approver for all operators rather than per-operator approvers.

## Complete Example

See the example repository:
- **k8s-apps-repo**: https://github.com/rajinator/k8s-apps-repo
- **ocp-gitops-repo**: https://github.com/rajinator/ocp-gitops-repo

**Key files:**
- `k8s-apps-repo/ocp/installplan-approver-operator/` - Operator deployment
- `ocp-gitops-repo/apps/components/installplan-approver-operator/` - ArgoCD Application
- `ocp-gitops-repo/apps/components/cert-manager-operator/` - Example managed operator

## Resources

- **ArgoCD Documentation**: https://argo-cd.readthedocs.io/
- **ArgoCD Health Checks**: https://argo-cd.readthedocs.io/en/stable/operator-manual/health/
- **GitOps Principles**: https://opengitops.dev/

