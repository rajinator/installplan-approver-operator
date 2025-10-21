#!/usr/bin/env bash

# Test script for InstallPlan Approver Operator
# This script helps you quickly test the operator locally

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

function log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

function log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

function check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    if ! command -v operator-sdk &> /dev/null; then
        log_error "operator-sdk is not installed or not in PATH"
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    log_info "All prerequisites met!"
}

function install_crds() {
    log_info "Installing CRDs..."
    make install
    log_info "CRDs installed successfully!"
}

function uninstall_crds() {
    log_info "Uninstalling CRDs..."
    make uninstall
    log_info "CRDs uninstalled successfully!"
}

function run_operator() {
    log_info "Running operator locally..."
    log_info "Press Ctrl+C to stop the operator"
    make run
}

function create_sample() {
    log_info "Creating sample InstallPlanApprover..."
    kubectl apply -f config/samples/operators_v1alpha1_installplanapprover.yaml
    log_info "Sample created successfully!"
    
    log_info "Checking InstallPlanApprover status..."
    kubectl get installplanapprovers
}

function delete_sample() {
    log_info "Deleting sample InstallPlanApprover..."
    kubectl delete -f config/samples/operators_v1alpha1_installplanapprover.yaml --ignore-not-found=true
    log_info "Sample deleted successfully!"
}

function create_test_namespace() {
    log_info "Creating test namespace..."
    kubectl create namespace test-operators --dry-run=client -o yaml | kubectl apply -f -
    log_info "Test namespace created!"
}

function create_test_installplan() {
    log_info "Creating test InstallPlan..."
    
    # Check if OLM is installed
    if ! kubectl get crd installplans.operators.coreos.com &> /dev/null; then
        log_warn "OLM is not installed. Skipping InstallPlan creation."
        log_warn "To install OLM, run: operator-sdk olm install"
        return
    fi
    
    # Create a test InstallPlan
    cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: InstallPlan
metadata:
  name: test-installplan
  namespace: test-operators
spec:
  approved: false
  clusterServiceVersionNames:
    - test-operator.v1.0.0
EOF
    
    log_info "Test InstallPlan created!"
    log_info "You can check it with: kubectl get installplan test-installplan -n test-operators"
}

function cleanup_test() {
    log_info "Cleaning up test resources..."
    kubectl delete installplan test-installplan -n test-operators --ignore-not-found=true
    kubectl delete namespace test-operators --ignore-not-found=true
    log_info "Test resources cleaned up!"
}

function watch_installplans() {
    local namespace=${1:-""}
    
    if [ -z "$namespace" ]; then
        log_info "Watching InstallPlans in all namespaces..."
        kubectl get installplans -A -w
    else
        log_info "Watching InstallPlans in namespace: $namespace"
        kubectl get installplans -n "$namespace" -w
    fi
}

function show_status() {
    log_info "Showing InstallPlanApprover status..."
    kubectl get installplanapprovers
    echo ""
    log_info "Detailed status:"
    kubectl get installplanapprovers -o yaml
}

function show_help() {
    cat <<EOF
InstallPlan Approver Operator - Test Helper Script

Usage: $0 <command>

Commands:
    check           Check prerequisites
    install         Install CRDs
    uninstall       Uninstall CRDs
    run             Run operator locally
    create-sample   Create sample InstallPlanApprover
    delete-sample   Delete sample InstallPlanApprover
    create-test     Create test namespace and InstallPlan
    cleanup-test    Clean up test resources
    watch           Watch InstallPlans in all namespaces
    watch-ns <ns>   Watch InstallPlans in specific namespace
    status          Show InstallPlanApprover status
    full-test       Run full test (install CRDs, create sample, run operator)
    help            Show this help message

Examples:
    # Quick start
    $0 full-test
    
    # Manual testing
    $0 check
    $0 install
    $0 run  # In one terminal
    $0 create-sample  # In another terminal
    $0 status
    
    # Test with mock InstallPlan
    $0 create-test
    $0 watch-ns test-operators

EOF
}

function full_test() {
    log_info "Running full test..."
    check_prerequisites
    install_crds
    create_sample
    create_test_namespace
    create_test_installplan
    log_info "Full test setup complete!"
    log_info "Now run the operator with: $0 run"
}

# Main script
case "${1:-}" in
    check)
        check_prerequisites
        ;;
    install)
        install_crds
        ;;
    uninstall)
        uninstall_crds
        ;;
    run)
        run_operator
        ;;
    create-sample)
        create_sample
        ;;
    delete-sample)
        delete_sample
        ;;
    create-test)
        create_test_namespace
        create_test_installplan
        ;;
    cleanup-test)
        cleanup_test
        ;;
    watch)
        watch_installplans
        ;;
    watch-ns)
        if [ -z "${2:-}" ]; then
            log_error "Please specify a namespace"
            exit 1
        fi
        watch_installplans "$2"
        ;;
    status)
        show_status
        ;;
    full-test)
        full_test
        ;;
    help|--help|-h|"")
        show_help
        ;;
    *)
        log_error "Unknown command: $1"
        show_help
        exit 1
        ;;
esac

