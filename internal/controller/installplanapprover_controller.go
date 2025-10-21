/*
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
*/

package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorsv1alpha1 "github.com/rajinator/installplan-approver-operator/api/v1alpha1"
)

// InstallPlanApproverReconciler reconciles a InstallPlanApprover object
type InstallPlanApproverReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=operators.bapu.cloud,resources=installplanapprovers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operators.bapu.cloud,resources=installplanapprovers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operators.bapu.cloud,resources=installplanapprovers/finalizers,verbs=update
// +kubebuilder:rbac:groups=operators.coreos.com,resources=installplans,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *InstallPlanApproverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the InstallPlanApprover instance
	approver := &operatorsv1alpha1.InstallPlanApprover{}
	err := r.Get(ctx, req.NamespacedName, approver)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			logger.Info("InstallPlanApprover resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get InstallPlanApprover")
		return ctrl.Result{}, err
	}

	// If auto-approve is disabled, skip processing
	if !approver.Spec.AutoApprove {
		logger.Info("Auto-approve is disabled, skipping InstallPlan processing")
		return ctrl.Result{}, nil
	}

	// Process InstallPlans in target namespaces
	namespaces := approver.Spec.TargetNamespaces
	if len(namespaces) == 0 {
		// If no target namespaces specified, list all namespaces
		namespaceList := &corev1.NamespaceList{}
		if err := r.List(ctx, namespaceList); err != nil {
			logger.Error(err, "Failed to list namespaces")
			return ctrl.Result{}, err
		}
		for _, ns := range namespaceList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}

	approvedCount := int32(0)
	var lastApprovedPlan string
	var lastApprovedTime *metav1.Time
	foundNonMatchingPlans := false

	// Process each namespace
	for _, namespace := range namespaces {
		count, lastPlan, lastTime, foundNonMatching, err := r.processNamespace(ctx, namespace, approver.Spec.OperatorNames)
		if err != nil {
			logger.Error(err, "Failed to process namespace", "namespace", namespace)
			continue
		}
		approvedCount += count
		if lastPlan != "" {
			lastApprovedPlan = fmt.Sprintf("%s/%s", namespace, lastPlan)
			lastApprovedTime = lastTime
		}
		if foundNonMatching {
			foundNonMatchingPlans = true
		}
	}

	// Update status if any InstallPlans were approved
	if approvedCount > 0 {
		approver.Status.ApprovedCount += approvedCount
		approver.Status.LastApprovedPlan = lastApprovedPlan
		approver.Status.LastApprovedTime = lastApprovedTime

		if err := r.Status().Update(ctx, approver); err != nil {
			logger.Error(err, "Failed to update InstallPlanApprover status")
			return ctrl.Result{}, err
		}
		logger.Info("Updated InstallPlanApprover status", "approvedCount", approvedCount)

		// If we approved something, let watches handle next reconciliation
		// No need to poll frequently since we'll get events when things change
		return ctrl.Result{}, nil
	}

	// Intelligent requeue based on what we found:
	// - If non-matching plans exist: wait 3 minutes (won't change without Subscription update)
	// - Otherwise: wait 1 minute (normal polling for new InstallPlans)
	var requeueAfter time.Duration
	if foundNonMatchingPlans {
		requeueAfter = 3 * time.Minute
		logger.V(1).Info("Found non-matching InstallPlans, requeuing with extended delay", "delay", requeueAfter)
	} else {
		requeueAfter = 1 * time.Minute
		logger.V(1).Info("No action taken, requeuing with normal polling delay", "delay", requeueAfter)
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// processNamespace processes InstallPlans in a given namespace
// Returns: (approvedCount, lastPlanName, lastApprovedTime, foundNonMatching, error)
func (r *InstallPlanApproverReconciler) processNamespace(ctx context.Context, namespace string, operatorNames []string) (int32, string, *metav1.Time, bool, error) {
	logger := log.FromContext(ctx)

	// Define the GVK for InstallPlan
	installPlanGVK := schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "InstallPlan",
	}

	// List InstallPlans using unstructured objects
	installPlanList := &unstructured.UnstructuredList{}
	installPlanList.SetGroupVersionKind(installPlanGVK)

	listOptions := &client.ListOptions{Namespace: namespace}
	if err := r.List(ctx, installPlanList, listOptions); err != nil {
		// If CRD doesn't exist, just log and continue
		if errors.IsNotFound(err) || runtime.IsNotRegisteredError(err) {
			logger.V(1).Info("InstallPlans CRD not found in cluster, skipping", "namespace", namespace)
			return 0, "", nil, false, nil
		}
		return 0, "", nil, false, err
	}

	approvedCount := int32(0)
	var lastApprovedPlan string
	var lastApprovedTime *metav1.Time
	foundNonMatching := false

	// Process each InstallPlan
	for _, item := range installPlanList.Items {
		// Get the approved field from spec
		approved, found, err := unstructured.NestedBool(item.Object, "spec", "approved")
		if err != nil {
			logger.Error(err, "Failed to get approved field", "installplan", item.GetName())
			continue
		}

		// If already approved, skip
		if found && approved {
			continue
		}

		// Check if operator name is in the allowed list (if specified)
		if len(operatorNames) > 0 {
			clusterServiceVersionNames, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "clusterServiceVersionNames")
			if !r.isOperatorAllowed(clusterServiceVersionNames, operatorNames) {
				logger.V(1).Info("Operator not in allowed list, skipping", "installplan", item.GetName())
				continue
			}
		}

		// NEW: Check if InstallPlan CSV matches Subscription's startingCSV
		// This prevents auto-approval of unintended upgrades
		if !r.matchesSubscriptionVersion(ctx, &item, namespace) {
			logger.Info("Skipping InstallPlan - CSV version doesn't match Subscription startingCSV",
				"installplan", item.GetName(), "namespace", namespace)
			foundNonMatching = true
			continue
		}

		// Approve the InstallPlan
		if err := unstructured.SetNestedField(item.Object, true, "spec", "approved"); err != nil {
			logger.Error(err, "Failed to set approved field", "installplan", item.GetName())
			continue
		}

		if err := r.Update(ctx, &item); err != nil {
			logger.Error(err, "Failed to approve InstallPlan", "installplan", item.GetName(), "namespace", namespace)
			continue
		}

		logger.Info("Approved InstallPlan", "installplan", item.GetName(), "namespace", namespace)
		approvedCount++
		lastApprovedPlan = item.GetName()
		now := metav1.Now()
		lastApprovedTime = &now
	}

	return approvedCount, lastApprovedPlan, lastApprovedTime, foundNonMatching, nil
}

// isOperatorAllowed checks if any CSV name matches the allowed operator names
func (r *InstallPlanApproverReconciler) isOperatorAllowed(csvNames []string, allowedNames []string) bool {
	if len(allowedNames) == 0 {
		return true
	}

	for _, csvName := range csvNames {
		for _, allowed := range allowedNames {
			if csvName == allowed || containsSubstring(csvName, allowed) {
				return true
			}
		}
	}
	return false
}

// containsSubstring checks if a string contains a substring
func containsSubstring(str, substr string) bool {
	return len(str) >= len(substr) && str[:len(substr)] == substr
}

// matchesSubscriptionVersion checks if the InstallPlan's CSV version matches the Subscription's startingCSV
// This prevents auto-approval of unintended upgrades while preserving Git-based version control
func (r *InstallPlanApproverReconciler) matchesSubscriptionVersion(ctx context.Context, installPlan *unstructured.Unstructured, namespace string) bool {
	logger := log.FromContext(ctx)

	// Get CSV names from InstallPlan
	csvNames, found, err := unstructured.NestedStringSlice(installPlan.Object, "spec", "clusterServiceVersionNames")
	if err != nil || !found || len(csvNames) == 0 {
		logger.V(1).Info("InstallPlan has no CSV names", "installplan", installPlan.GetName())
		return true // If no CSV specified, allow (shouldn't happen in practice)
	}

	// Get the primary CSV (usually the first one)
	installPlanCSV := csvNames[0]

	// Find the owning Subscription by checking ownerReferences
	ownerRefs := installPlan.GetOwnerReferences()
	var subscriptionName string
	for _, owner := range ownerRefs {
		if owner.Kind == "Subscription" {
			subscriptionName = owner.Name
			break
		}
	}

	if subscriptionName == "" {
		logger.V(1).Info("InstallPlan has no Subscription owner", "installplan", installPlan.GetName())
		return true // If no owner, allow (manual InstallPlan or edge case)
	}

	// Fetch the Subscription
	subscriptionGVK := schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "Subscription",
	}

	subscription := &unstructured.Unstructured{}
	subscription.SetGroupVersionKind(subscriptionGVK)
	subscriptionKey := types.NamespacedName{
		Name:      subscriptionName,
		Namespace: namespace,
	}

	if err := r.Get(ctx, subscriptionKey, subscription); err != nil {
		logger.Error(err, "Failed to get Subscription", "subscription", subscriptionName, "namespace", namespace)
		return true // On error, allow (fail open to avoid blocking valid installs)
	}

	// Get startingCSV from Subscription
	startingCSV, found, err := unstructured.NestedString(subscription.Object, "spec", "startingCSV")
	if err != nil || !found || startingCSV == "" {
		logger.V(1).Info("Subscription has no startingCSV", "subscription", subscriptionName)
		return true // If no startingCSV specified, allow (Subscription will use latest)
	}

	// Compare versions
	if installPlanCSV == startingCSV {
		logger.Info("InstallPlan CSV matches Subscription startingCSV - approving",
			"installplan", installPlan.GetName(),
			"csv", installPlanCSV,
			"subscription", subscriptionName)
		return true
	}

	logger.Info("InstallPlan CSV does not match Subscription startingCSV - skipping",
		"installplan", installPlan.GetName(),
		"installPlanCSV", installPlanCSV,
		"subscriptionStartingCSV", startingCSV,
		"subscription", subscriptionName)
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstallPlanApproverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Define the GVK for InstallPlan
	installPlanGVK := schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "InstallPlan",
	}

	// Create an unstructured object for InstallPlan watching
	installPlanUnstructured := &unstructured.Unstructured{}
	installPlanUnstructured.SetGroupVersionKind(installPlanGVK)

	// Create a new controller with watches
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorsv1alpha1.InstallPlanApprover{}).
		Watches(
			installPlanUnstructured,
			handler.EnqueueRequestsFromMapFunc(r.findApproversForInstallPlan),
		).
		Named("installplanapprover").
		Complete(r)
}

// findApproversForInstallPlan maps InstallPlans to InstallPlanApprover objects
func (r *InstallPlanApproverReconciler) findApproversForInstallPlan(ctx context.Context, obj client.Object) []reconcile.Request {
	// List all InstallPlanApprover resources
	approverList := &operatorsv1alpha1.InstallPlanApproverList{}
	if err := r.List(ctx, approverList); err != nil {
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0, len(approverList.Items))
	for _, approver := range approverList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      approver.GetName(),
				Namespace: approver.GetNamespace(),
			},
		})
	}
	return requests
}
