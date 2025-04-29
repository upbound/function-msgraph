// Package v1beta1 contains the input type for this Function
// +kubebuilder:object:generate=true
// +groupName=msgraph.fn.crossplane.io
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

	// QueryType defines the type of Microsoft Graph API query to perform
	// Supported values: UserValidation, GroupMembership, GroupObjectIDs, ServicePrincipalDetails
	QueryType string `json:"queryType"`

	// Users is a list of userPrincipalName (email IDs) for user validation
	// +optional
	Users []*string `json:"users,omitempty"`

	// Groups is a list of group names for group object ID queries
	// +optional
	Groups []*string `json:"groups,omitempty"`

	// Group is a single group name for group membership queries
	// +optional
	Group *string `json:"group,omitempty"`

	// GroupRef is a reference to retrieve the group name (e.g., from status or context)
	// Overrides Group field if used
	// +optional
	GroupRef *string `json:"groupRef,omitempty"`

	// ServicePrincipals is a list of service principal names
	// +optional
	ServicePrincipals []*string `json:"servicePrincipals,omitempty"`

	// Target where to store the Query Result
	Target string `json:"target"`

	// SkipQueryWhenTargetHasData controls whether to skip the query when the target already has data
	// Default is false to ensure continuous reconciliation
	// +optional
	SkipQueryWhenTargetHasData *bool `json:"skipQueryWhenTargetHasData,omitempty"`
}
