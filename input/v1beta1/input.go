// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=approve.fn.crossplane.io
// +versionName=v1alpha1
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This isn't a custom resource, in the sense that we never install its CRD.
// It is a KRM-like object, so we generate a CRD to describe its schema.

// Input can be used to provide input to this Function.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type Input struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// DataField defines the object field to hash and store for tracking changes
	// For example: "spec.resources"
	DataField string `json:"dataField"`

	// HashAlgorithm defines which hash algorithm to use for calculating hashes
	// Supported values: "md5", "sha256", "sha512"
	// Default is "sha256"
	// +optional
	HashAlgorithm *string `json:"hashAlgorithm,omitempty"`

	// ApprovalField defines the status field to check for the approval decision
	// Default is "status.approved"
	// +optional
	ApprovalField *string `json:"approvalField,omitempty"`

	// OldHashField defines where to store the previous (approved) hash value
	// Default is "status.oldHash"
	// +optional
	OldHashField *string `json:"oldHashField,omitempty"`

	// NewHashField defines where to store the current hash value
	// Default is "status.newHash"
	// +optional
	NewHashField *string `json:"newHashField,omitempty"`

	// PauseAnnotation defines which annotation to use for pausing reconciliation
	// Default is "crossplane.io/paused"
	// +optional
	PauseAnnotation *string `json:"pauseAnnotation,omitempty"`

	// DetailedCondition adds a detailed condition about approval status
	// Default is true 
	// +optional
	DetailedCondition *bool `json:"detailedCondition,omitempty"`

	// ApprovalMessage sets a message to display when approval is required
	// Default is "Changes detected. Approval required."
	// +optional
	ApprovalMessage *string `json:"approvalMessage,omitempty"`
}
