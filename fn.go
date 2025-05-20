package main

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"hash"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/upbound/function-approve/input/v1beta1"
)

// Function implements the manual approval workflow function.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(ctx context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	// Parse input and get defaults
	in, err := f.parseInput(req, rsp)
	if err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp
	}

	// Initialize response with desired XR and preserve context
	if err := f.initializeResponse(req, rsp); err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp
	}

	// Extract data to hash
	dataToHash, err := f.extractDataToHash(req, in, rsp)
	if err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp
	}

	// Calculate hash
	newHash := f.calculateHash(dataToHash, in)

	// Get old hash from status
	oldHash, err := f.getOldHash(req, in, rsp)
	if err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp
	}

	// Save new hash to status
	if err := f.saveNewHash(req, in, newHash, rsp); err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp
	}

	// Check approval status
	approved, err := f.checkApprovalStatus(req, in, rsp)
	if err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp
	}

	// Reconciliation pause logic
	if !approved || (oldHash != "" && oldHash != newHash) {
		// Changes detected and not approved, pause reconciliation
		if err := f.pauseReconciliation(req, in, rsp); err != nil {
			return rsp, nil //nolint:nilerr // errors are handled in rsp
		}

		// Set condition to show approval is needed
		msg := "Changes detected. Approval required."
		if in.ApprovalMessage != nil {
			msg = *in.ApprovalMessage
		}

		condition := response.ConditionFalse(rsp, "ApprovalRequired", "WaitingForApproval").
			WithMessage(msg).
			TargetCompositeAndClaim()

		if in.DetailedCondition != nil && *in.DetailedCondition {
			// Add detailed information about what changed and what needs approval
			detailedMsg := msg + "\nCurrent hash: " + newHash + "\n" +
				"Approved hash: " + oldHash + "\n" +
				"Approve this change by setting " + *in.ApprovalField + " to true"
			condition = condition.WithMessage(detailedMsg)
		}

		// Return early, pausing reconciliation
		return rsp, nil
	}

	// If we got here, the changes are approved or there are no changes
	// Update the old hash with the new hash
	if err := f.saveOldHash(req, in, newHash, rsp); err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp
	}

	// Remove pause annotation if it exists
	if err := f.resumeReconciliation(req, in, rsp); err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp
	}

	// Set success condition
	response.ConditionTrue(rsp, "FunctionSuccess", "Success").
		WithMessage("Approved successfully").
		TargetCompositeAndClaim()

	return rsp, nil
}

// parseInput parses the function input and sets defaults.
func (f *Function) parseInput(req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse) (*v1beta1.Input, error) {
	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get Function input"))
		return nil, err
	}

	// Set default values if not provided
	if in.HashAlgorithm == nil {
		defaultAlgo := "sha256"
		in.HashAlgorithm = &defaultAlgo
	}

	if in.ApprovalField == nil {
		defaultField := "status.approved"
		in.ApprovalField = &defaultField
	}

	if in.OldHashField == nil {
		defaultField := "status.oldHash"
		in.OldHashField = &defaultField
	}

	if in.NewHashField == nil {
		defaultField := "status.newHash"
		in.NewHashField = &defaultField
	}

	if in.PauseAnnotation == nil {
		defaultAnnotation := "crossplane.io/paused"
		in.PauseAnnotation = &defaultAnnotation
	}

	if in.DetailedCondition == nil {
		defaultValue := true
		in.DetailedCondition = &defaultValue
	}

	return in, nil
}

// initializeResponse initializes the response with desired XR and preserves context
func (f *Function) initializeResponse(req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse) error {
	// Ensure oxr to dxr gets propagated and we keep status around
	if err := f.propagateDesiredXR(req, rsp); err != nil {
		return err
	}
	
	// Ensure the context is preserved
	f.preserveContext(req, rsp)
	
	return nil
}

// getXRAndStatus retrieves status and desired XR, handling initialization if needed
func (f *Function) getXRAndStatus(req *fnv1.RunFunctionRequest) (map[string]interface{}, *resource.Composite, error) {
	// Get composite resources
	oxr, dxr, err := f.getObservedAndDesired(req)
	if err != nil {
		return nil, nil, err
	}

	// Initialize and copy data
	f.initializeAndCopyData(oxr, dxr)

	// Get status
	xrStatus := f.getStatusFromResources(oxr, dxr)

	return xrStatus, dxr, nil
}

// getObservedAndDesired gets both observed and desired XR resources
func (f *Function) getObservedAndDesired(req *fnv1.RunFunctionRequest) (*resource.Composite, *resource.Composite, error) {
	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot get observed composite resource")
	}

	dxr, err := request.GetDesiredCompositeResource(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot get desired composite resource")
	}

	return oxr, dxr, nil
}

// initializeAndCopyData initializes metadata and copies spec
func (f *Function) initializeAndCopyData(oxr, dxr *resource.Composite) {
	// Initialize dxr from oxr if needed
	if dxr.Resource.GetKind() == "" {
		dxr.Resource.SetAPIVersion(oxr.Resource.GetAPIVersion())
		dxr.Resource.SetKind(oxr.Resource.GetKind())
		dxr.Resource.SetName(oxr.Resource.GetName())
	}

	// Copy spec from observed to desired XR to preserve it
	xrSpec := make(map[string]interface{})
	if err := oxr.Resource.GetValueInto("spec", &xrSpec); err == nil && len(xrSpec) > 0 {
		if err := dxr.Resource.SetValue("spec", xrSpec); err != nil {
			f.log.Debug("Cannot set spec in desired XR", "error", err)
		}
	}
}

// getStatusFromResources gets status from desired or observed XR
func (f *Function) getStatusFromResources(oxr, dxr *resource.Composite) map[string]interface{} {
	xrStatus := make(map[string]interface{})

	// First try to get status from desired XR (pipeline changes)
	if dxr.Resource.GetKind() != "" {
		err := dxr.Resource.GetValueInto("status", &xrStatus)
		if err == nil && len(xrStatus) > 0 {
			return xrStatus
		}
		f.log.Debug("Cannot get status from Desired XR or it's empty")
	}

	// Fallback to observed XR status
	err := oxr.Resource.GetValueInto("status", &xrStatus)
	if err != nil {
		f.log.Debug("Cannot get status from Observed XR")
	}

	return xrStatus
}

// propagateDesiredXR ensures the desired XR is properly propagated without changing existing data
func (f *Function) propagateDesiredXR(req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse) error {
	xrStatus, dxr, err := f.getXRAndStatus(req)
	if err != nil {
		response.Fatal(rsp, err)
		return err
	}

	// Write any existing status back to dxr
	if len(xrStatus) > 0 {
		if err := dxr.Resource.SetValue("status", xrStatus); err != nil {
			f.log.Info("Error setting status in Desired XR", "error", err)
			return err
		}
	}

	// Save the desired XR in the response
	if err := response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composite resource in %T", rsp))
		return err
	}

	f.log.Info("Successfully propagated Desired XR")
	return nil
}

// preserveContext ensures the context is preserved in the response
func (f *Function) preserveContext(req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse) {
	// Get the existing context from the request
	existingContext := req.GetContext()
	if existingContext != nil {
		// Copy the existing context to the response
		rsp.Context = existingContext
		f.log.Info("Preserved existing context in response")
	}
}

// ParseNestedKey enables the bracket and dot notation to key reference
func ParseNestedKey(key string) ([]string, error) {
	var parts []string
	// Regular expression to extract keys, supporting both dot and bracket notation
	// For simplicity in this implementation, we'll use a simpler approach
	for _, part := range strings.Split(key, ".") {
		if part != "" {
			parts = append(parts, part)
		}
	}

	if len(parts) == 0 {
		return nil, errors.New("invalid key")
	}
	return parts, nil
}

// GetNestedValue retrieves a nested value from a map using dot notation keys.
func GetNestedValue(data map[string]interface{}, key string) (interface{}, bool, error) {
	parts, err := ParseNestedKey(key)
	if err != nil {
		return nil, false, err
	}

	currentValue := interface{}(data)
	for _, k := range parts {
		// Check if the current value is a map
		if nestedMap, ok := currentValue.(map[string]interface{}); ok {
			// Get the next value in the nested map
			if nextValue, exists := nestedMap[k]; exists {
				currentValue = nextValue
			} else {
				return nil, false, nil
			}
		} else {
			return nil, false, nil
		}
	}

	return currentValue, true, nil
}

// SetNestedValue sets a value to a nested key from a map using dot notation keys.
func SetNestedValue(root map[string]interface{}, key string, value interface{}) error {
	parts, err := ParseNestedKey(key)
	if err != nil {
		return err
	}

	current := root
	for i, part := range parts {
		if i == len(parts)-1 {
			// Set the value at the final key
			current[part] = value
			return nil
		}

		// Traverse into nested maps or create them if they don't exist
		if next, exists := current[part]; exists {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return errors.Errorf("key %q exists but is not a map", part)
			}
		} else {
			// Create a new map if the path doesn't exist
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}

	return nil
}

// extractDataToHash extracts the data to hash from the specified field
func (f *Function) extractDataToHash(req *fnv1.RunFunctionRequest, in *v1beta1.Input, rsp *fnv1.RunFunctionResponse) (interface{}, error) {
	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get observed composite resource"))
		return nil, err
	}

	// Determine if we're looking in spec or status
	parts := strings.SplitN(in.DataField, ".", 2)
	if len(parts) != 2 {
		response.Fatal(rsp, errors.Errorf("invalid DataField format: %s, expected section.field (e.g. spec.resources)", in.DataField))
		return nil, errors.New("invalid DataField format")
	}

	section, field := parts[0], parts[1]
	
	var sectionData map[string]interface{}
	if err := oxr.Resource.GetValueInto(section, &sectionData); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get %s from observed XR", section))
		return nil, err
	}

	data, exists, err := GetNestedValue(sectionData, field)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "error accessing field %s", field))
		return nil, err
	}

	if !exists {
		response.Fatal(rsp, errors.Errorf("field %s.%s not found in resource", section, field))
		return nil, errors.New("field not found")
	}

	return data, nil
}

// calculateHash calculates hash for the given data using the specified algorithm
func (f *Function) calculateHash(data interface{}, in *v1beta1.Input) string {
	// Create a JSON representation of the data
	jsonData, err := json.Marshal(data)
	if err != nil {
		f.log.Debug("Error marshaling data to JSON", "error", err)
		return ""
	}

	// Choose hash algorithm
	var h hash.Hash
	switch *in.HashAlgorithm {
	case "md5":
		h = md5.New()
	case "sha512":
		h = sha512.New()
	default:
		// Default to sha256
		h = sha256.New()
	}

	// Calculate hash
	h.Write(jsonData)
	return hex.EncodeToString(h.Sum(nil))
}

// getOldHash retrieves the previously approved hash
func (f *Function) getOldHash(req *fnv1.RunFunctionRequest, in *v1beta1.Input, rsp *fnv1.RunFunctionResponse) (string, error) {
	xrStatus, _, err := f.getXRAndStatus(req)
	if err != nil {
		response.Fatal(rsp, err)
		return "", err
	}

	// Remove status. prefix if present
	hashField := *in.OldHashField
	if strings.HasPrefix(hashField, "status.") {
		hashField = strings.TrimPrefix(hashField, "status.")
	}

	// Get the old hash from status
	value, exists, err := GetNestedValue(xrStatus, hashField)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "error accessing old hash field %s", hashField))
		return "", err
	}

	if !exists {
		// Not an error, just means this is the first time we're seeing this resource
		return "", nil
	}

	strValue, ok := value.(string)
	if !ok {
		response.Fatal(rsp, errors.Errorf("old hash field %s is not a string", hashField))
		return "", errors.New("old hash field is not a string")
	}

	return strValue, nil
}

// saveNewHash saves the new hash to the status
func (f *Function) saveNewHash(req *fnv1.RunFunctionRequest, in *v1beta1.Input, hash string, rsp *fnv1.RunFunctionResponse) error {
	xrStatus, dxr, err := f.getXRAndStatus(req)
	if err != nil {
		response.Fatal(rsp, err)
		return err
	}

	// Remove status. prefix if present
	hashField := *in.NewHashField
	if strings.HasPrefix(hashField, "status.") {
		hashField = strings.TrimPrefix(hashField, "status.")
	}

	// Set the new hash in status
	if err := SetNestedValue(xrStatus, hashField, hash); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set new hash field %s", hashField))
		return err
	}

	// Write updated status back to dxr
	if err := dxr.Resource.SetValue("status", xrStatus); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot write updated status back into composite resource"))
		return err
	}

	// Save the updated desired composite resource
	if err := response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composite resource in %T", rsp))
		return err
	}

	return nil
}

// saveOldHash updates the old hash with the new hash after approval
func (f *Function) saveOldHash(req *fnv1.RunFunctionRequest, in *v1beta1.Input, hash string, rsp *fnv1.RunFunctionResponse) error {
	xrStatus, dxr, err := f.getXRAndStatus(req)
	if err != nil {
		response.Fatal(rsp, err)
		return err
	}

	// Remove status. prefix if present
	hashField := *in.OldHashField
	if strings.HasPrefix(hashField, "status.") {
		hashField = strings.TrimPrefix(hashField, "status.")
	}

	// Set the old hash in status
	if err := SetNestedValue(xrStatus, hashField, hash); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set old hash field %s", hashField))
		return err
	}

	// Write updated status back to dxr
	if err := dxr.Resource.SetValue("status", xrStatus); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot write updated status back into composite resource"))
		return err
	}

	// Save the updated desired composite resource
	if err := response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composite resource in %T", rsp))
		return err
	}

	return nil
}

// checkApprovalStatus checks if the current changes are approved
func (f *Function) checkApprovalStatus(req *fnv1.RunFunctionRequest, in *v1beta1.Input, rsp *fnv1.RunFunctionResponse) (bool, error) {
	xrStatus, _, err := f.getXRAndStatus(req)
	if err != nil {
		response.Fatal(rsp, err)
		return false, err
	}

	// Remove status. prefix if present
	approvalField := *in.ApprovalField
	if strings.HasPrefix(approvalField, "status.") {
		approvalField = strings.TrimPrefix(approvalField, "status.")
	}

	// Get the approval status
	value, exists, err := GetNestedValue(xrStatus, approvalField)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "error accessing approval field %s", approvalField))
		return false, err
	}

	if !exists {
		// Not explicitly approved
		return false, nil
	}

	boolValue, ok := value.(bool)
	if !ok {
		response.Fatal(rsp, errors.Errorf("approval field %s is not a boolean", approvalField))
		return false, errors.New("approval field is not a boolean")
	}

	return boolValue, nil
}

// pauseReconciliation adds a pause annotation to the resource
func (f *Function) pauseReconciliation(req *fnv1.RunFunctionRequest, in *v1beta1.Input, rsp *fnv1.RunFunctionResponse) error {
	_, dxr, err := f.getXRAndStatus(req)
	if err != nil {
		response.Fatal(rsp, err)
		return err
	}

	// Get current annotations
	annotations := dxr.Resource.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	// Add pause annotation
	annotations[*in.PauseAnnotation] = "true"
	dxr.Resource.SetAnnotations(annotations)

	// Save the updated desired composite resource
	if err := response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composite resource in %T", rsp))
		return err
	}

	return nil
}

// resumeReconciliation removes the pause annotation from the resource
func (f *Function) resumeReconciliation(req *fnv1.RunFunctionRequest, in *v1beta1.Input, rsp *fnv1.RunFunctionResponse) error {
	_, dxr, err := f.getXRAndStatus(req)
	if err != nil {
		response.Fatal(rsp, err)
		return err
	}

	// Get current annotations
	annotations := dxr.Resource.GetAnnotations()
	if annotations == nil || annotations[*in.PauseAnnotation] == "" {
		// No pause annotation, nothing to do
		return nil
	}

	// Remove pause annotation
	delete(annotations, *in.PauseAnnotation)
	dxr.Resource.SetAnnotations(annotations)

	// Save the updated desired composite resource
	if err := response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composite resource in %T", rsp))
		return err
	}

	return nil
}