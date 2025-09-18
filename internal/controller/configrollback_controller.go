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
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	configv1alpha1 "github.com/rahulkshinde/k8s-config-controller/api/v1alpha1"
)

const (
	// ConfigRollbackFinalizer is the finalizer used by the controller
	ConfigRollbackFinalizer = "config.rahulkshinde.dev/finalizer"

	// Condition types
	ConditionTypeAvailable   = "Available"
	ConditionTypeProgressing = "Progressing"
	ConditionTypeDegraded    = "Degraded"

	// Health check statuses
	HealthStatusHealthy   = "Healthy"
	HealthStatusUnhealthy = "Unhealthy"
	HealthStatusUnknown   = "Unknown"

	// Rollback statuses
	RollbackStatusNotAttempted = "NotAttempted"
	RollbackStatusInProgress   = "InProgress"
	RollbackStatusSuccess      = "Success"
	RollbackStatusFailed       = "Failed"
)

// ConfigRollbackReconciler reconciles a ConfigRollback object
type ConfigRollbackReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	HTTPClient *http.Client
}

// +kubebuilder:rbac:groups=config.rahulkshinde.dev,resources=configrollbacks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.rahulkshinde.dev,resources=configrollbacks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=config.rahulkshinde.dev,resources=configrollbacks/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ConfigRollbackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the ConfigRollback instance
	var configRollback configv1alpha1.ConfigRollback
	if err := r.Get(ctx, req.NamespacedName, &configRollback); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ConfigRollback resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ConfigRollback")
		return ctrl.Result{}, err
	}

	log.Info("Processing ConfigRollback", "name", configRollback.Name, "namespace", configRollback.Namespace)

	// Handle deletion
	if configRollback.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &configRollback)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&configRollback, ConfigRollbackFinalizer) {
		controllerutil.AddFinalizer(&configRollback, ConfigRollbackFinalizer)
		if err := r.Update(ctx, &configRollback); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Skip processing if disabled
	if !configRollback.Spec.Enabled {
		log.Info("ConfigRollback is disabled, skipping processing")
		return r.updateStatus(ctx, &configRollback, HealthStatusUnknown, "ConfigRollback is disabled")
	}

	// Update observed generation
	configRollback.Status.ObservedGeneration = configRollback.Generation

	// Perform health check
	healthStatus, healthMessage := r.performHealthCheck(ctx, &configRollback)

	// Update consecutive failures counter
	if healthStatus == HealthStatusUnhealthy {
		configRollback.Status.ConsecutiveFailures++
	} else if healthStatus == HealthStatusHealthy {
		configRollback.Status.ConsecutiveFailures = 0
		configRollback.Status.RollbackAttempts = 0 // Reset rollback attempts on recovery
	}

	// Check if rollback is needed
	needsRollback := configRollback.Status.ConsecutiveFailures >= configRollback.Spec.HealthChecks.FailureThreshold

	if needsRollback && configRollback.Status.RollbackAttempts < configRollback.Spec.RollbackConfig.MaxRollbackAttempts {
		log.Info("Health check failure threshold reached, initiating rollback",
			"consecutiveFailures", configRollback.Status.ConsecutiveFailures,
			"threshold", configRollback.Spec.HealthChecks.FailureThreshold)

		rollbackStatus, rollbackMessage := r.performRollback(ctx, &configRollback)
		configRollback.Status.RollbackStatus = rollbackStatus
		configRollback.Status.LastRollbackMessage = rollbackMessage
		configRollback.Status.RollbackAttempts++
		now := metav1.Now()
		configRollback.Status.LastRollbackTime = &now
	}

	// Update status
	result, err := r.updateStatus(ctx, &configRollback, healthStatus, healthMessage)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Calculate next reconciliation interval
	interval := time.Duration(configRollback.Spec.MonitoringInterval) * time.Second
	log.V(1).Info("Scheduling next reconciliation", "interval", interval)

	return ctrl.Result{RequeueAfter: interval}, nil
}

// handleDeletion handles the cleanup when a ConfigRollback is being deleted
func (r *ConfigRollbackReconciler) handleDeletion(ctx context.Context, configRollback *configv1alpha1.ConfigRollback) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Perform any cleanup logic here
	log.Info("Cleaning up ConfigRollback resources")

	// Remove finalizer
	controllerutil.RemoveFinalizer(configRollback, ConfigRollbackFinalizer)
	if err := r.Update(ctx, configRollback); err != nil {
		log.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// performHealthCheck executes the configured health checks
func (r *ConfigRollbackReconciler) performHealthCheck(ctx context.Context, configRollback *configv1alpha1.ConfigRollback) (string, string) {
	log := logf.FromContext(ctx)

	// Update last health check time
	now := metav1.Now()
	configRollback.Status.LastHealthCheckTime = &now

	healthConfig := configRollback.Spec.HealthChecks

	// Perform HTTP health check if configured
	if healthConfig.HTTPHealthCheck != nil {
		return r.performHTTPHealthCheck(ctx, configRollback)
	}

	// Perform Prometheus health check if configured
	if healthConfig.PrometheusHealthCheck != nil {
		return r.performPrometheusHealthCheck(ctx, configRollback)
	}

	log.Info("No health check configuration found")
	return HealthStatusUnknown, "No health check configuration specified"
}

// performHTTPHealthCheck executes HTTP-based health checks
func (r *ConfigRollbackReconciler) performHTTPHealthCheck(ctx context.Context, configRollback *configv1alpha1.ConfigRollback) (string, string) {
	log := logf.FromContext(ctx)

	target := configRollback.Spec.TargetService
	httpCheck := configRollback.Spec.HealthChecks.HTTPHealthCheck
	timeout := time.Duration(configRollback.Spec.HealthChecks.TimeoutSeconds) * time.Second

	// Construct health check URL
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s",
		target.Name, target.Namespace, target.Port, httpCheck.Path)

	log.Info("Performing HTTP health check", "url", url)

	// Create HTTP request with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Error(err, "Failed to create HTTP request")
		return HealthStatusUnhealthy, fmt.Sprintf("Failed to create HTTP request: %v", err)
	}

	// Add custom headers
	for key, value := range httpCheck.Headers {
		req.Header.Set(key, value)
	}

	// Perform the request
	client := r.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error(err, "HTTP health check failed")
		return HealthStatusUnhealthy, fmt.Sprintf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	expectedCode := httpCheck.ExpectedStatusCode
	if expectedCode == 0 {
		expectedCode = 200 // Default
	}

	if resp.StatusCode == int(expectedCode) {
		log.Info("HTTP health check passed", "statusCode", resp.StatusCode)
		return HealthStatusHealthy, fmt.Sprintf("HTTP health check passed (status: %d)", resp.StatusCode)
	}

	log.Info("HTTP health check failed", "expectedCode", expectedCode, "actualCode", resp.StatusCode)
	return HealthStatusUnhealthy, fmt.Sprintf("HTTP health check failed: expected %d, got %d", expectedCode, resp.StatusCode)
}

// performPrometheusHealthCheck executes Prometheus metrics-based health checks
func (r *ConfigRollbackReconciler) performPrometheusHealthCheck(ctx context.Context, configRollback *configv1alpha1.ConfigRollback) (string, string) {
	log := logf.FromContext(ctx)

	// This is a placeholder implementation
	// In a real implementation, you would:
	// 1. Query the Prometheus API with the configured query
	// 2. Parse the response and extract metric values
	// 3. Compare against the threshold using the specified operator

	promCheck := configRollback.Spec.HealthChecks.PrometheusHealthCheck
	log.Info("Prometheus health check not yet implemented",
		"prometheusURL", promCheck.PrometheusURL,
		"query", promCheck.Query,
		"threshold", promCheck.Threshold)

	return HealthStatusUnknown, "Prometheus health check not yet implemented"
}

// performRollback executes the configured rollback mechanism
func (r *ConfigRollbackReconciler) performRollback(ctx context.Context, configRollback *configv1alpha1.ConfigRollback) (string, string) {
	log := logf.FromContext(ctx)

	rollbackConfig := configRollback.Spec.RollbackConfig

	switch rollbackConfig.RollbackType {
	case "webhook":
		return r.performWebhookRollback(ctx, configRollback)
	case "lambda":
		return r.performLambdaRollback(ctx, configRollback)
	case "gitops":
		return r.performGitOpsRollback(ctx, configRollback)
	default:
		log.Error(nil, "Unknown rollback type", "type", rollbackConfig.RollbackType)
		return RollbackStatusFailed, fmt.Sprintf("Unknown rollback type: %s", rollbackConfig.RollbackType)
	}
}

// performWebhookRollback executes webhook-based rollbacks
func (r *ConfigRollbackReconciler) performWebhookRollback(ctx context.Context, configRollback *configv1alpha1.ConfigRollback) (string, string) {
	log := logf.FromContext(ctx)

	// This is a placeholder implementation
	// In a real implementation, you would make HTTP requests to the configured webhook

	webhookConfig := configRollback.Spec.RollbackConfig.WebhookConfig
	if webhookConfig == nil {
		return RollbackStatusFailed, "Webhook configuration is missing"
	}

	log.Info("Webhook rollback simulated", "url", webhookConfig.URL)
	return RollbackStatusSuccess, "Webhook rollback simulated successfully"
}

// performLambdaRollback executes AWS Lambda-based rollbacks
func (r *ConfigRollbackReconciler) performLambdaRollback(ctx context.Context, configRollback *configv1alpha1.ConfigRollback) (string, string) {
	log := logf.FromContext(ctx)

	// This is a placeholder implementation
	// In a real implementation, you would use AWS SDK to invoke the Lambda function

	lambdaConfig := configRollback.Spec.RollbackConfig.LambdaConfig
	if lambdaConfig == nil {
		return RollbackStatusFailed, "Lambda configuration is missing"
	}

	log.Info("Lambda rollback simulated",
		"functionName", lambdaConfig.FunctionName,
		"region", lambdaConfig.Region)
	return RollbackStatusSuccess, "Lambda rollback simulated successfully"
}

// performGitOpsRollback executes GitOps-based rollbacks
func (r *ConfigRollbackReconciler) performGitOpsRollback(ctx context.Context, configRollback *configv1alpha1.ConfigRollback) (string, string) {
	log := logf.FromContext(ctx)

	// This is a placeholder implementation
	// In a real implementation, you would:
	// 1. Clone the Git repository
	// 2. Modify the configuration file to rollback to the previous version
	// 3. Commit and push the changes

	gitOpsConfig := configRollback.Spec.RollbackConfig.GitOpsConfig
	if gitOpsConfig == nil {
		return RollbackStatusFailed, "GitOps configuration is missing"
	}

	log.Info("GitOps rollback simulated",
		"repository", gitOpsConfig.Repository,
		"filePath", gitOpsConfig.FilePath,
		"previousVersion", gitOpsConfig.PreviousVersion)
	return RollbackStatusSuccess, "GitOps rollback simulated successfully"
}

// updateStatus updates the ConfigRollback status with health and condition information
func (r *ConfigRollbackReconciler) updateStatus(ctx context.Context, configRollback *configv1alpha1.ConfigRollback, healthStatus, healthMessage string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Update health status
	configRollback.Status.HealthCheckStatus = healthStatus

	// Update conditions based on health status
	var condition metav1.Condition
	switch healthStatus {
	case HealthStatusHealthy:
		condition = metav1.Condition{
			Type:               ConditionTypeAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "HealthCheckPassed",
			Message:            healthMessage,
			LastTransitionTime: metav1.Now(),
		}
	case HealthStatusUnhealthy:
		condition = metav1.Condition{
			Type:               ConditionTypeDegraded,
			Status:             metav1.ConditionTrue,
			Reason:             "HealthCheckFailed",
			Message:            healthMessage,
			LastTransitionTime: metav1.Now(),
		}
	default:
		condition = metav1.Condition{
			Type:               ConditionTypeProgressing,
			Status:             metav1.ConditionTrue,
			Reason:             "HealthCheckUnknown",
			Message:            healthMessage,
			LastTransitionTime: metav1.Now(),
		}
	}

	// Update or add the condition
	r.setCondition(&configRollback.Status.Conditions, condition)

	// Update the status
	if err := r.Status().Update(ctx, configRollback); err != nil {
		log.Error(err, "Failed to update ConfigRollback status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// setCondition updates or adds a condition to the conditions slice
func (r *ConfigRollbackReconciler) setCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	for i, condition := range *conditions {
		if condition.Type == newCondition.Type {
			(*conditions)[i] = newCondition
			return
		}
	}
	*conditions = append(*conditions, newCondition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigRollbackReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1alpha1.ConfigRollback{}).
		Named("configrollback").
		Complete(r)
}
