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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ConfigRollbackSpec defines the desired state of ConfigRollback
type ConfigRollbackSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// targetService specifies the service to monitor for health checks
	// +kubebuilder:validation:Required
	TargetService ServiceTarget `json:"targetService"`

	// healthChecks defines the health monitoring configuration
	// +kubebuilder:validation:Required
	HealthChecks HealthCheckConfig `json:"healthChecks"`

	// rollbackConfig defines how rollbacks should be executed
	// +kubebuilder:validation:Required
	RollbackConfig RollbackConfiguration `json:"rollbackConfig"`

	// monitoringInterval defines how often to perform health checks (in seconds)
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=3600
	// +kubebuilder:default=30
	MonitoringInterval int32 `json:"monitoringInterval,omitempty"`

	// enabled allows temporarily disabling the rollback monitoring
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`
}

// ServiceTarget defines the target service to monitor
type ServiceTarget struct {
	// name of the service to monitor
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// namespace of the service
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`

	// port to use for health checks
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}

// HealthCheckConfig defines health monitoring parameters
type HealthCheckConfig struct {
	// httpHealthCheck defines HTTP-based health checking
	// +optional
	HTTPHealthCheck *HTTPHealthCheck `json:"httpHealthCheck,omitempty"`

	// prometheusHealthCheck defines Prometheus metrics-based health checking
	// +optional
	PrometheusHealthCheck *PrometheusHealthCheck `json:"prometheusHealthCheck,omitempty"`

	// failureThreshold defines how many consecutive failures trigger a rollback
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=3
	FailureThreshold int32 `json:"failureThreshold,omitempty"`

	// timeoutSeconds defines the timeout for each health check
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=300
	// +kubebuilder:default=10
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

// HTTPHealthCheck defines HTTP-based health check parameters
type HTTPHealthCheck struct {
	// path for the health check endpoint
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// expectedStatusCode defines the expected HTTP status code for healthy response
	// +kubebuilder:validation:Minimum=200
	// +kubebuilder:validation:Maximum=299
	// +kubebuilder:default=200
	ExpectedStatusCode int32 `json:"expectedStatusCode,omitempty"`

	// headers to include in the health check request
	// +optional
	Headers map[string]string `json:"headers,omitempty"`
}

// PrometheusHealthCheck defines Prometheus metrics-based health checking
type PrometheusHealthCheck struct {
	// prometheusURL is the URL of the Prometheus server
	// +kubebuilder:validation:Required
	PrometheusURL string `json:"prometheusURL"`

	// query is the PromQL query to evaluate service health
	// +kubebuilder:validation:Required
	Query string `json:"query"`

	// threshold defines the threshold value for the metric (as string to avoid float precision issues)
	// +kubebuilder:validation:Required
	Threshold string `json:"threshold"`

	// operator defines how to compare the metric value with threshold (>, <, >=, <=, ==, !=)
	// +kubebuilder:validation:Enum=gt;lt;gte;lte;eq;ne
	// +kubebuilder:default="gt"
	Operator string `json:"operator,omitempty"`
}

// RollbackConfiguration defines how rollbacks should be executed
type RollbackConfiguration struct {
	// rollbackType defines the type of rollback mechanism
	// +kubebuilder:validation:Enum=webhook;lambda;gitops
	// +kubebuilder:validation:Required
	RollbackType string `json:"rollbackType"`

	// webhookConfig defines webhook-based rollback configuration
	// +optional
	WebhookConfig *WebhookRollbackConfig `json:"webhookConfig,omitempty"`

	// lambdaConfig defines AWS Lambda-based rollback configuration
	// +optional
	LambdaConfig *LambdaRollbackConfig `json:"lambdaConfig,omitempty"`

	// gitOpsConfig defines GitOps-based rollback configuration
	// +optional
	GitOpsConfig *GitOpsRollbackConfig `json:"gitOpsConfig,omitempty"`

	// maxRollbackAttempts defines maximum number of rollback attempts
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default=1
	MaxRollbackAttempts int32 `json:"maxRollbackAttempts,omitempty"`
}

// WebhookRollbackConfig defines webhook-based rollback parameters
type WebhookRollbackConfig struct {
	// url is the webhook URL to call for rollback
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// method is the HTTP method to use (POST, PUT, PATCH)
	// +kubebuilder:validation:Enum=POST;PUT;PATCH
	// +kubebuilder:default="POST"
	Method string `json:"method,omitempty"`

	// headers to include in the webhook request
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// payload is the JSON payload to send
	// +optional
	Payload string `json:"payload,omitempty"`
}

// LambdaRollbackConfig defines AWS Lambda-based rollback parameters
type LambdaRollbackConfig struct {
	// functionName is the AWS Lambda function name
	// +kubebuilder:validation:Required
	FunctionName string `json:"functionName"`

	// region is the AWS region
	// +kubebuilder:validation:Required
	Region string `json:"region"`

	// payload is the JSON payload to send to Lambda
	// +optional
	Payload string `json:"payload,omitempty"`
}

// GitOpsRollbackConfig defines GitOps-based rollback parameters
type GitOpsRollbackConfig struct {
	// repository is the Git repository URL
	// +kubebuilder:validation:Required
	Repository string `json:"repository"`

	// branch is the Git branch to use
	// +kubebuilder:default="main"
	Branch string `json:"branch,omitempty"`

	// filePath is the path to the configuration file to modify
	// +kubebuilder:validation:Required
	FilePath string `json:"filePath"`

	// previousVersion is the version to rollback to
	// +kubebuilder:validation:Required
	PreviousVersion string `json:"previousVersion"`
}

// ConfigRollbackStatus defines the observed state of ConfigRollback.
type ConfigRollbackStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the ConfigRollback resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// lastHealthCheckTime indicates when the last health check was performed
	// +optional
	LastHealthCheckTime *metav1.Time `json:"lastHealthCheckTime,omitempty"`

	// healthCheckStatus indicates the current health status of the monitored service
	// +kubebuilder:validation:Enum=Healthy;Unhealthy;Unknown
	// +optional
	HealthCheckStatus string `json:"healthCheckStatus,omitempty"`

	// consecutiveFailures tracks the number of consecutive health check failures
	// +optional
	ConsecutiveFailures int32 `json:"consecutiveFailures,omitempty"`

	// lastRollbackTime indicates when the last rollback was attempted
	// +optional
	LastRollbackTime *metav1.Time `json:"lastRollbackTime,omitempty"`

	// rollbackAttempts tracks the number of rollback attempts for the current failure
	// +optional
	RollbackAttempts int32 `json:"rollbackAttempts,omitempty"`

	// rollbackStatus indicates the status of the most recent rollback attempt
	// +kubebuilder:validation:Enum=Success;Failed;InProgress;NotAttempted
	// +optional
	RollbackStatus string `json:"rollbackStatus,omitempty"`

	// lastRollbackMessage provides details about the last rollback attempt
	// +optional
	LastRollbackMessage string `json:"lastRollbackMessage,omitempty"`

	// observedGeneration reflects the generation of the most recently observed ConfigRollback
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ConfigRollback is the Schema for the configrollbacks API
type ConfigRollback struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of ConfigRollback
	// +required
	Spec ConfigRollbackSpec `json:"spec"`

	// status defines the observed state of ConfigRollback
	// +optional
	Status ConfigRollbackStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// ConfigRollbackList contains a list of ConfigRollback
type ConfigRollbackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigRollback `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigRollback{}, &ConfigRollbackList{})
}
