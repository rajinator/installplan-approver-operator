# Development Guide

Guide for developing and contributing to the InstallPlan Approver Operator.

## Prerequisites

### Required Tools

- **Go 1.24.3+**: [Install Go](https://golang.org/doc/install)
- **Operator SDK v1.41.1+**: [Install Operator SDK](https://sdk.operatorframework.io/docs/installation/)
- **kubectl**: [Install kubectl](https://kubernetes.io/docs/tasks/tools/)
- **Docker** or **Podman**: For building images
- **Make**: Usually pre-installed on Linux/macOS
- **Git**: For version control

### Development Cluster

You'll need access to a Kubernetes cluster with OLM installed:

- **Kind** (recommended for local development)
- **Minikube**
- **OpenShift CodeReady Containers (CRC)**
- **Remote cluster** with kubeconfig access

## Project Setup

### Clone Repository

```bash
git clone https://github.com/rajinator/installplan-approver-operator.git
cd installplan-approver-operator
```

### Install Dependencies

```bash
go mod download
go mod tidy
```

### Install OLM (if not already installed)

```bash
operator-sdk olm install
```

## Local Development

### 1. Install CRDs

```bash
make install
```

This installs the `InstallPlanApprover` CRD to your cluster.

### 2. Run Operator Locally

```bash
make run
```

The operator runs on your local machine but connects to your kubeconfig cluster.

**You'll see:**
```
INFO  Starting Controller	controller=installplanapprover
INFO  Starting workers	controller=installplanapprover worker count=1
```

### 3. Create Test Resources

In another terminal:

```bash
# Create a test namespace
kubectl create namespace test-operators

# Create a Subscription with Manual approval
cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: cert-manager
  namespace: test-operators
spec:
  channel: stable
  name: cert-manager
  source: operatorhubio-catalog
  sourceNamespace: olm
  installPlanApproval: Manual
  startingCSV: cert-manager.v1.15.0
EOF

# Create an InstallPlanApprover
kubectl apply -f config/samples/operators_v1alpha1_installplanapprover.yaml
```

### 4. Watch Operator Logs

The operator should detect the InstallPlan and approve it.

### 5. Make Changes

Edit code in `internal/controller/` or `api/v1alpha1/`.

**After changes:**
```bash
# Regenerate code
make generate

# Regenerate manifests
make manifests

# Restart operator (Ctrl+C and run again)
make run
```

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### E2E Tests

```bash
# Build and deploy to cluster
make docker-build docker-push IMG=<your-registry>/installplan-approver-operator:test
make deploy IMG=<your-registry>/installplan-approver-operator:test

# Run e2e tests
make test-e2e
```

### Linting

```bash
# Run golangci-lint
golangci-lint run

# Or use make target (if configured)
make lint
```

## Building

### Build Binary

```bash
# Build for your platform
make build

# Binary created at: bin/manager
```

### Build Container Image

```bash
# Build for your platform
make docker-build IMG=<your-registry>/installplan-approver-operator:latest

# Push to registry
make docker-push IMG=<your-registry>/installplan-approver-operator:latest
```

### Multi-Architecture Build

```bash
# Build for amd64 and arm64
docker buildx build --platform linux/amd64,linux/arm64 \
  -t <your-registry>/installplan-approver-operator:latest \
  --push .
```

## Makefile Targets

Common make targets:

| Target | Description |
|--------|-------------|
| `make manifests` | Generate CRD and RBAC manifests |
| `make generate` | Generate code (DeepCopy, etc.) |
| `make fmt` | Format Go code |
| `make vet` | Run go vet |
| `make test` | Run unit tests |
| `make build` | Build operator binary |
| `make run` | Run operator locally |
| `make install` | Install CRDs to cluster |
| `make uninstall` | Uninstall CRDs from cluster |
| `make deploy` | Deploy operator to cluster |
| `make undeploy` | Remove operator from cluster |
| `make docker-build` | Build container image |
| `make docker-push` | Push container image |

## Project Structure

```
.
├── api/v1alpha1/                    # API definitions
│   ├── installplanapprover_types.go # CRD types
│   └── zz_generated.deepcopy.go     # Generated code
├── cmd/
│   └── main.go                      # Main entry point
├── config/                          # Kubernetes manifests
│   ├── crd/                         # CRD definitions
│   ├── default/                     # Default kustomization
│   ├── manager/                     # Manager deployment
│   ├── rbac/                        # RBAC roles and bindings
│   └── samples/                     # Sample CRs
├── docs/                            # Documentation
├── internal/controller/             # Controller implementation
│   ├── installplanapprover_controller.go      # Main reconciliation logic
│   └── installplanapprover_controller_test.go # Unit tests
├── test/e2e/                        # E2E tests
├── Dockerfile                       # Container image build
├── go.mod                           # Go module definition
├── go.sum                           # Go dependencies
├── Makefile                         # Build automation
└── PROJECT                          # Kubebuilder project metadata
```

## Code Guidelines

### Go Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Run `golangci-lint` before committing
- Write meaningful comments for exported functions
- Use descriptive variable names

### Commit Messages

Follow conventional commits:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Test changes
- `chore`: Build/tooling changes

**Examples:**
```
feat(controller): add operator name filtering

Add support for filtering InstallPlans by operator name prefix.
This allows users to target specific operators while watching
all namespaces.

Closes #123
```

```
fix(rbac): add Subscription read permissions

The operator needs to read Subscriptions to match CSV versions.
Added get/list/watch permissions for Subscriptions.

Fixes #456
```

### Testing Guidelines

- Write unit tests for all new functionality
- Aim for >80% code coverage
- Use table-driven tests where appropriate
- Mock external dependencies

**Example:**
```go
func TestShouldApproveInstallPlan(t *testing.T) {
    tests := []struct {
        name     string
        planCSV  string
        subCSV   string
        expected bool
    }{
        {"exact match", "cert-manager.v1.15.0", "cert-manager.v1.15.0", true},
        {"version mismatch", "cert-manager.v1.16.0", "cert-manager.v1.15.0", false},
        {"no startingCSV", "cert-manager.v1.15.0", "", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test logic
        })
    }
}
```

## Debugging

### Debug with Delve

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Run with debugger
dlv debug ./cmd/main.go
```

### Debug in VSCode

`.vscode/launch.json`:
```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug Operator",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/cmd/main.go",
            "args": [],
            "env": {
                "KUBECONFIG": "${env:HOME}/.kube/config"
            }
        }
    ]
}
```

### Enable Verbose Logging

```go
// In controller code
logger.V(1).Info("Debug message", "key", value)
```

Run with:
```bash
make run ARGS="--zap-log-level=debug"
```

## Contributing

### Workflow

1. **Fork the repository**
2. **Create a feature branch**
   ```bash
   git checkout -b feature/my-feature
   ```
3. **Make changes**
4. **Add tests**
5. **Run tests**
   ```bash
   make test
   ```
6. **Commit changes**
   ```bash
   git commit -m "feat: add my feature"
   ```
7. **Push to your fork**
   ```bash
   git push origin feature/my-feature
   ```
8. **Create Pull Request**

### Pull Request Guidelines

- Include tests for new features
- Update documentation
- Follow code style guidelines
- Keep PRs focused (one feature/fix per PR)
- Reference related issues

### Code Review Process

1. Automated checks run (tests, linting)
2. Maintainer reviews code
3. Address feedback
4. Approval and merge

## Release Process

### Versioning

We use [Semantic Versioning](https://semver.org/):

- **MAJOR**: Incompatible API changes
- **MINOR**: New features (backwards compatible)
- **PATCH**: Bug fixes (backwards compatible)

### Creating a Release

1. **Update version**
   ```bash
   # Update VERSION file or Makefile
   echo "v0.2.0" > VERSION
   ```

2. **Update CHANGELOG.md**

3. **Commit changes**
   ```bash
   git commit -am "chore: release v0.2.0"
   ```

4. **Create tag**
   ```bash
   git tag -a v0.2.0 -m "Release v0.2.0"
   git push origin v0.2.0
   ```

5. **GitHub Actions builds and pushes images**

6. **Create GitHub Release** with release notes

## Continuous Integration

### GitHub Actions

Workflows in `.github/workflows/`:

- **docker-build-push.yml**: Builds and pushes container images
- **release.yml**: Creates GitHub Releases on tags
- **test.yml**: Runs tests on PRs (if configured)

### Local CI Testing

```bash
# Install act (https://github.com/nektos/act)
act -l  # List workflows
act push  # Run push workflow
```

## Resources

- **Operator SDK Documentation**: https://sdk.operatorframework.io/docs/
- **Kubebuilder Book**: https://book.kubebuilder.io/
- **OLM Documentation**: https://olm.operatorframework.io/
- **Kubernetes API Conventions**: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md

