package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	azauth "github.com/microsoft/kiota-authentication-azure-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/groups"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/serviceprincipals"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
	"github.com/upbound/function-msgraph/input/v1beta1"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

// GraphQueryInterface defines the methods required for querying Microsoft Graph API.
type GraphQueryInterface interface {
	graphQuery(ctx context.Context, azureCreds map[string]string, in *v1beta1.Input) (interface{}, error)
}

// Function returns whatever response you ask it to.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	graphQuery GraphQueryInterface

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(ctx context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	// Ensure oxr to dxr gets propagated and we keep status around
	if err := f.propagateDesiredXR(req, rsp); err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp. We should not error main function and proceed with reconciliation
	}
	// Ensure the context is preserved
	f.preserveContext(req, rsp)

	// Parse input and get credentials
	in, azureCreds, err := f.parseInputAndCredentials(req, rsp)
	if err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp. We should not error main function and proceed with reconciliation
	}

	// Check if target is valid
	if !f.isValidTarget(in.Target) {
		response.Fatal(rsp, errors.Errorf("Unrecognized target field: %s", in.Target))
		return rsp, nil
	}

	// Check if we should skip the query
	if f.shouldSkipQuery(req, in, rsp) {
		// Set success condition
		response.ConditionTrue(rsp, "FunctionSuccess", "Success").
			TargetCompositeAndClaim()
		return rsp, nil
	}

	// Execute the query
	results, err := f.executeQuery(ctx, azureCreds, in, rsp)
	if err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp. We should not error main function and proceed with reconciliation
	}

	// Process the results
	if err := f.processResults(req, in, results, rsp); err != nil {
		return rsp, nil //nolint:nilerr // errors are handled in rsp. We should not error main function and proceed with reconciliation
	}

	// Set success condition
	response.ConditionTrue(rsp, "FunctionSuccess", "Success").
		TargetCompositeAndClaim()

	return rsp, nil
}

// parseInputAndCredentials parses the input and gets the credentials.
func (f *Function) parseInputAndCredentials(req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse) (*v1beta1.Input, map[string]string, error) {
	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").
			WithMessage("Something went wrong.").
			TargetCompositeAndClaim()

		response.Warning(rsp, errors.New("something went wrong")).
			TargetCompositeAndClaim()

		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return nil, nil, err
	}

	azureCreds, err := getCreds(req)
	if err != nil {
		response.Fatal(rsp, err)
		return nil, nil, err
	}

	if f.graphQuery == nil {
		f.graphQuery = &GraphQuery{}
	}

	return in, azureCreds, nil
}

// getXRAndStatus retrieves status and desired XR, handling initialization if needed
func (f *Function) getXRAndStatus(req *fnv1.RunFunctionRequest) (map[string]interface{}, *resource.Composite, error) {
	// Get both observed and desired XR
	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot get observed composite resource")
	}

	dxr, err := request.GetDesiredCompositeResource(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot get desired composite resource")
	}

	xrStatus := make(map[string]interface{})

	// Initialize dxr from oxr if needed
	if dxr.Resource.GetKind() == "" {
		dxr.Resource.SetAPIVersion(oxr.Resource.GetAPIVersion())
		dxr.Resource.SetKind(oxr.Resource.GetKind())
		dxr.Resource.SetName(oxr.Resource.GetName())
	}

	// First try to get status from desired XR (pipeline changes)
	if dxr.Resource.GetKind() != "" {
		err = dxr.Resource.GetValueInto("status", &xrStatus)
		if err == nil && len(xrStatus) > 0 {
			return xrStatus, dxr, nil
		}
		f.log.Debug("Cannot get status from Desired XR or it's empty")
	}

	// Fallback to observed XR status
	err = oxr.Resource.GetValueInto("status", &xrStatus)
	if err != nil {
		f.log.Debug("Cannot get status from Observed XR")
	}

	return xrStatus, dxr, nil
}

// checkStatusTargetHasData checks if the status target has data.
func (f *Function) checkStatusTargetHasData(req *fnv1.RunFunctionRequest, in *v1beta1.Input, rsp *fnv1.RunFunctionResponse) bool {
	xrStatus, _, err := f.getXRAndStatus(req)
	if err != nil {
		response.Fatal(rsp, err)
		return true
	}

	statusField := strings.TrimPrefix(in.Target, "status.")
	if hasData, _ := targetHasData(xrStatus, statusField); hasData {
		f.log.Info("Target already has data, skipping query", "target", in.Target)
		response.ConditionTrue(rsp, "FunctionSkip", "SkippedQuery").
			WithMessage("Target already has data, skipped query to avoid throttling").
			TargetCompositeAndClaim()
		return true
	}
	return false
}

// executeQuery executes the query.
func (f *Function) executeQuery(ctx context.Context, azureCreds map[string]string, in *v1beta1.Input, rsp *fnv1.RunFunctionResponse) (interface{}, error) {
	// Initialize GraphQuery with logger if needed
	if gq, ok := f.graphQuery.(*GraphQuery); ok {
		gq.log = f.log
	}

	results, err := f.graphQuery.graphQuery(ctx, azureCreds, in)
	if err != nil {
		response.Fatal(rsp, err)
		f.log.Info("FAILURE: ", "failure", fmt.Sprint(err))
		return nil, err
	}

	// Print the obtained query results
	f.log.Info("Query Type:", "queryType", in.QueryType)
	f.log.Info("Results:", "results", fmt.Sprint(results))
	response.Normalf(rsp, "QueryType: %q", in.QueryType)

	return results, nil
}

// processResults processes the query results.
func (f *Function) processResults(req *fnv1.RunFunctionRequest, in *v1beta1.Input, results interface{}, rsp *fnv1.RunFunctionResponse) error {
	switch {
	case strings.HasPrefix(in.Target, "status."):
		err := f.putQueryResultToStatus(req, rsp, in, results)
		if err != nil {
			response.Fatal(rsp, err)
			return err
		}
	case strings.HasPrefix(in.Target, "context."):
		err := putQueryResultToContext(req, rsp, in, results, f)
		if err != nil {
			response.Fatal(rsp, err)
			return err
		}
	default:
		// This should never happen because we check for valid targets earlier
		response.Fatal(rsp, errors.Errorf("Unrecognized target field: %s", in.Target))
		return errors.New("unrecognized target field")
	}
	return nil
}

func getCreds(req *fnv1.RunFunctionRequest) (map[string]string, error) {
	var azureCreds map[string]string
	rawCreds := req.GetCredentials()

	if credsData, ok := rawCreds["azure-creds"]; ok {
		credsData := credsData.GetCredentialData().GetData()
		if credsJSON, ok := credsData["credentials"]; ok {
			err := json.Unmarshal(credsJSON, &azureCreds)
			if err != nil {
				return nil, errors.Wrap(err, "cannot parse json credentials")
			}
		}
	} else {
		return nil, errors.New("failed to get azure-creds credentials")
	}

	return azureCreds, nil
}

// GraphQuery is a concrete implementation of the GraphQueryInterface
// that interacts with Microsoft Graph API.
type GraphQuery struct {
	log logging.Logger
}

// createGraphClient initializes a Microsoft Graph client using the provided credentials
func (g *GraphQuery) createGraphClient(azureCreds map[string]string) (*msgraphsdk.GraphServiceClient, error) {
	tenantID := azureCreds["tenantId"]
	clientID := azureCreds["clientId"]
	clientSecret := azureCreds["clientSecret"]

	// Create Azure credential for Microsoft Graph
	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain credentials")
	}

	// Create authentication provider
	authProvider, err := azauth.NewAzureIdentityAuthenticationProviderWithScopes(cred, []string{"https://graph.microsoft.com/.default"})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create auth provider")
	}

	// Create adapter
	adapter, err := msgraphsdk.NewGraphRequestAdapter(authProvider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create graph adapter")
	}

	// Initialize Microsoft Graph client
	return msgraphsdk.NewGraphServiceClient(adapter), nil
}

// graphQuery is a concrete implementation that interacts with Microsoft Graph API.
func (g *GraphQuery) graphQuery(ctx context.Context, azureCreds map[string]string, in *v1beta1.Input) (interface{}, error) {
	// Create the Microsoft Graph client
	client, err := g.createGraphClient(azureCreds)
	if err != nil {
		return nil, err
	}

	// Route based on query type
	switch in.QueryType {
	case "UserValidation":
		return g.validateUsers(ctx, client, in)
	case "GroupMembership":
		return g.getGroupMembers(ctx, client, in)
	case "GroupObjectIDs":
		return g.getGroupObjectIDs(ctx, client, in)
	case "ServicePrincipalDetails":
		return g.getServicePrincipalDetails(ctx, client, in)
	default:
		return nil, errors.Errorf("unsupported query type: %s", in.QueryType)
	}
}

// validateUsers validates if the provided user principal names (emails) exist
func (g *GraphQuery) validateUsers(ctx context.Context, client *msgraphsdk.GraphServiceClient, in *v1beta1.Input) (interface{}, error) {
	if len(in.Users) == 0 {
		return nil, errors.New("no users provided for validation")
	}

	var results []interface{}

	for _, userPrincipalName := range in.Users {
		if userPrincipalName == nil {
			continue
		}

		// Create request configuration
		requestConfig := &users.UsersRequestBuilderGetRequestConfiguration{
			QueryParameters: &users.UsersRequestBuilderGetQueryParameters{},
		}

		// Build filter expression
		filterValue := fmt.Sprintf("userPrincipalName eq '%s'", *userPrincipalName)
		requestConfig.QueryParameters.Filter = &filterValue

		// Use standard fields for user validation
		requestConfig.QueryParameters.Select = []string{"id", "displayName", "userPrincipalName", "mail"}

		// Execute the query
		result, err := client.Users().Get(ctx, requestConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to validate user %s", *userPrincipalName)
		}

		// Process results
		if result.GetValue() != nil {
			for _, user := range result.GetValue() {
				userMap := map[string]interface{}{
					"id":                user.GetId(),
					"displayName":       user.GetDisplayName(),
					"userPrincipalName": user.GetUserPrincipalName(),
					"mail":              user.GetMail(),
				}
				results = append(results, userMap)
			}
		}
	}

	return results, nil
}

// findGroupByName finds a group by its display name and returns its ID
func (g *GraphQuery) findGroupByName(ctx context.Context, client *msgraphsdk.GraphServiceClient, groupName string) (*string, error) {
	// Create filter by displayName
	filterValue := fmt.Sprintf("displayName eq '%s'", groupName)
	groupRequestConfig := &groups.GroupsRequestBuilderGetRequestConfiguration{
		QueryParameters: &groups.GroupsRequestBuilderGetQueryParameters{
			Filter: &filterValue,
		},
	}

	// Query for the group
	groupResult, err := client.Groups().Get(ctx, groupRequestConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find group")
	}

	// Verify we found a group
	if groupResult.GetValue() == nil || len(groupResult.GetValue()) == 0 {
		return nil, errors.Errorf("group not found: %s", groupName)
	}

	// Return the group ID
	return groupResult.GetValue()[0].GetId(), nil
}

// fetchGroupMembers fetches all members of a group by group ID
func (g *GraphQuery) fetchGroupMembers(ctx context.Context, client *msgraphsdk.GraphServiceClient, groupID string, groupName string) ([]models.DirectoryObjectable, error) {
	// Create a request configuration that expands members
	// This is the workaround for the known issue where service principals
	// are not listed as group members in v1.0
	// See: https://developer.microsoft.com/en-us/graph/known-issues/?search=25984
	requestConfig := &groups.GroupItemRequestBuilderGetRequestConfiguration{
		QueryParameters: &groups.GroupItemRequestBuilderGetQueryParameters{
			Expand: []string{"members"},
		},
	}

	// Get the group with expanded members using the workaround
	// mentioned in the Microsoft documentation
	group, err := client.Groups().ByGroupId(groupID).Get(ctx, requestConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get members for group %s", groupName)
	}

	// Extract the members from the expanded result
	var members []models.DirectoryObjectable
	if group.GetMembers() != nil {
		members = group.GetMembers()
	}

	// Log basic information about the membership
	if g.log != nil {
		g.log.Debug("Retrieved group members", "groupName", groupName, "groupID", groupID, "memberCount", len(members))
	}

	return members, nil
}

// extractDisplayName attempts to extract the display name from a directory object
func (g *GraphQuery) extractDisplayName(member models.DirectoryObjectable, memberID string) string {
	additionalData := member.GetAdditionalData()

	// Try to get from additional data first
	if displayNameVal, exists := additionalData["displayName"]; exists && displayNameVal != nil {
		if displayName, ok := displayNameVal.(string); ok {
			return displayName
		}
	}

	// Try to use reflection to call GetDisplayName if it exists
	memberValue := reflect.ValueOf(member)
	displayNameMethod := memberValue.MethodByName("GetDisplayName")
	if displayNameMethod.IsValid() && displayNameMethod.Type().NumIn() == 0 {
		results := displayNameMethod.Call(nil)
		if len(results) > 0 && !results[0].IsNil() {
			// Check if the result is a *string
			if displayNamePtr, ok := results[0].Interface().(*string); ok && displayNamePtr != nil {
				return *displayNamePtr
			}
		}
	}

	// Use fallback display name
	return fmt.Sprintf("Member %s", memberID)
}

// extractStringProperty safely extracts a string property from additionalData
func (g *GraphQuery) extractStringProperty(additionalData map[string]interface{}, key string) (string, bool) {
	if val, exists := additionalData[key]; exists && val != nil {
		if strVal, ok := val.(string); ok {
			return strVal, true
		}
	}
	return "", false
}

// extractUserProperties extracts user-specific properties from additionalData
func (g *GraphQuery) extractUserProperties(additionalData map[string]interface{}, memberMap map[string]interface{}) {
	// Extract mail property
	if mail, ok := g.extractStringProperty(additionalData, "mail"); ok {
		memberMap["mail"] = mail
	}

	// Extract userPrincipalName property
	if upn, ok := g.extractStringProperty(additionalData, "userPrincipalName"); ok {
		memberMap["userPrincipalName"] = upn
	}
}

// extractServicePrincipalProperties extracts service principal specific properties
func (g *GraphQuery) extractServicePrincipalProperties(additionalData map[string]interface{}, memberMap map[string]interface{}) {
	// Extract appId property
	if appID, ok := g.extractStringProperty(additionalData, "appId"); ok {
		memberMap["appId"] = appID
	}
}

// processMember extracts member information into a map
func (g *GraphQuery) processMember(member models.DirectoryObjectable) map[string]interface{} {
	// Define constants for member types
	const (
		userType             = "user"
		servicePrincipalType = "servicePrincipal"
		unknownType          = "unknown"
	)

	memberID := member.GetId()
	additionalData := member.GetAdditionalData()

	// Create basic member info
	memberMap := map[string]interface{}{
		"id": memberID,
	}

	// Determine member type
	memberType := unknownType

	// Check properties that indicate user type
	_, hasUserPrincipalName := g.extractStringProperty(additionalData, "userPrincipalName")
	_, hasMail := g.extractStringProperty(additionalData, "mail")
	if hasUserPrincipalName || hasMail {
		memberType = userType
	}

	// Check properties that indicate service principal type
	_, hasAppID := g.extractStringProperty(additionalData, "appId")
	if hasAppID {
		memberType = servicePrincipalType
	}

	// Try interface type checking for more accuracy
	if _, ok := member.(models.Userable); ok {
		memberType = userType
	}
	if _, ok := member.(models.ServicePrincipalable); ok {
		memberType = servicePrincipalType
	}

	// Add type to member info
	memberMap["type"] = memberType

	// Extract display name
	memberMap["displayName"] = g.extractDisplayName(member, *memberID)

	// Extract type-specific properties
	switch memberType {
	case userType:
		g.extractUserProperties(additionalData, memberMap)
	case servicePrincipalType:
		g.extractServicePrincipalProperties(additionalData, memberMap)
	}

	return memberMap
}

// getGroupMembers retrieves all members of the specified group
func (g *GraphQuery) getGroupMembers(ctx context.Context, client *msgraphsdk.GraphServiceClient, in *v1beta1.Input) (interface{}, error) {
	// Validate input
	if in.Group == nil || *in.Group == "" {
		return nil, errors.New("no group name provided")
	}

	// Find the group
	groupID, err := g.findGroupByName(ctx, client, *in.Group)
	if err != nil {
		return nil, err
	}

	// Fetch the members
	memberObjects, err := g.fetchGroupMembers(ctx, client, *groupID, *in.Group)
	if err != nil {
		return nil, err
	}

	// Process the members
	members := make([]interface{}, 0, len(memberObjects))
	for _, member := range memberObjects {
		memberMap := g.processMember(member)
		members = append(members, memberMap)
	}

	return members, nil
}

// getGroupObjectIDs retrieves object IDs for the specified group names
func (g *GraphQuery) getGroupObjectIDs(ctx context.Context, client *msgraphsdk.GraphServiceClient, in *v1beta1.Input) (interface{}, error) {
	if len(in.Groups) == 0 {
		return nil, errors.New("no group names provided")
	}

	var results []interface{}

	for _, groupName := range in.Groups {
		if groupName == nil {
			continue
		}

		// Create request configuration
		requestConfig := &groups.GroupsRequestBuilderGetRequestConfiguration{
			QueryParameters: &groups.GroupsRequestBuilderGetQueryParameters{},
		}

		// Find the group by displayName
		filterValue := fmt.Sprintf("displayName eq '%s'", *groupName)
		requestConfig.QueryParameters.Filter = &filterValue

		// Use standard fields for group object IDs
		requestConfig.QueryParameters.Select = []string{"id", "displayName", "description"}

		groupResult, err := client.Groups().Get(ctx, requestConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find group %s", *groupName)
		}

		if groupResult.GetValue() != nil && len(groupResult.GetValue()) > 0 {
			for _, group := range groupResult.GetValue() {
				groupMap := map[string]interface{}{
					"id":          group.GetId(),
					"displayName": group.GetDisplayName(),
					"description": group.GetDescription(),
				}
				results = append(results, groupMap)
			}
		}
	}

	return results, nil
}

// getServicePrincipalDetails retrieves details about service principals by name
func (g *GraphQuery) getServicePrincipalDetails(ctx context.Context, client *msgraphsdk.GraphServiceClient, in *v1beta1.Input) (interface{}, error) {
	if len(in.ServicePrincipals) == 0 {
		return nil, errors.New("no service principal names provided")
	}

	var results []interface{}

	for _, spName := range in.ServicePrincipals {
		if spName == nil {
			continue
		}

		// Create request configuration
		requestConfig := &serviceprincipals.ServicePrincipalsRequestBuilderGetRequestConfiguration{
			QueryParameters: &serviceprincipals.ServicePrincipalsRequestBuilderGetQueryParameters{},
		}

		// Find service principal by displayName
		filterValue := fmt.Sprintf("displayName eq '%s'", *spName)
		requestConfig.QueryParameters.Filter = &filterValue

		// Use standard fields for service principals
		requestConfig.QueryParameters.Select = []string{"id", "appId", "displayName", "description"}

		spResult, err := client.ServicePrincipals().Get(ctx, requestConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find service principal %s", *spName)
		}

		if spResult.GetValue() != nil && len(spResult.GetValue()) > 0 {
			for _, sp := range spResult.GetValue() {
				spMap := map[string]interface{}{
					"id":          sp.GetId(),
					"appId":       sp.GetAppId(),
					"displayName": sp.GetDisplayName(),
					"description": sp.GetDescription(),
				}
				results = append(results, spMap)
			}
		}
	}

	return results, nil
}

// ParseNestedKey enables the bracket and dot notation to key reference
func ParseNestedKey(key string) ([]string, error) {
	var parts []string
	// Regular expression to extract keys, supporting both dot and bracket notation
	regex := regexp.MustCompile(`\[([^\[\]]+)\]|([^.\[\]]+)`)
	matches := regex.FindAllStringSubmatch(key, -1)
	for _, match := range matches {
		if match[1] != "" {
			parts = append(parts, match[1]) // Bracket notation
		} else if match[2] != "" {
			parts = append(parts, match[2]) // Dot notation
		}
	}

	if len(parts) == 0 {
		return nil, errors.New("invalid key")
	}
	return parts, nil
}

// GetNestedKey retrieves a nested string value from a map using dot notation keys.
func GetNestedKey(context map[string]interface{}, key string) (string, bool) {
	parts, err := ParseNestedKey(key)
	if err != nil {
		return "", false
	}

	currentValue := interface{}(context)
	for _, k := range parts {
		// Check if the current value is a map
		if nestedMap, ok := currentValue.(map[string]interface{}); ok {
			// Get the next value in the nested map
			if nextValue, exists := nestedMap[k]; exists {
				currentValue = nextValue
			} else {
				return "", false
			}
		} else {
			return "", false
		}
	}

	// Convert the final value to a string
	if result, ok := currentValue.(string); ok {
		return result, true
	}
	return "", false
}

// SetNestedKey sets a value to a nested key from a map using dot notation keys.
func SetNestedKey(root map[string]interface{}, key string, value interface{}) error {
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
				return fmt.Errorf("key %q exists but is not a map", part)
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

// putQueryResultToStatus processes the query results to status
func (f *Function) putQueryResultToStatus(req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse, in *v1beta1.Input, results interface{}) error {
	xrStatus, dxr, err := f.getXRAndStatus(req)
	if err != nil {
		return err
	}

	// Update the specific status field
	statusField := strings.TrimPrefix(in.Target, "status.")
	err = SetNestedKey(xrStatus, statusField, results)
	if err != nil {
		return errors.Wrapf(err, "cannot set status field %s to %v", statusField, results)
	}

	// Write the updated status field back into the composite resource
	if err := dxr.Resource.SetValue("status", xrStatus); err != nil {
		return errors.Wrap(err, "cannot write updated status back into composite resource")
	}

	// Save the updated desired composite resource
	if err := response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		return errors.Wrapf(err, "cannot set desired composite resource in %T", rsp)
	}
	return nil
}

func putQueryResultToContext(req *fnv1.RunFunctionRequest, rsp *fnv1.RunFunctionResponse, in *v1beta1.Input, results interface{}, f *Function) error {
	contextField := strings.TrimPrefix(in.Target, "context.")
	data, err := structpb.NewValue(results)
	if err != nil {
		return errors.Wrap(err, "cannot convert results data to structpb.Value")
	}

	// Convert existing context into a map[string]interface{}
	contextMap := req.GetContext().AsMap()

	err = SetNestedKey(contextMap, contextField, data.AsInterface())
	if err != nil {
		return errors.Wrap(err, "failed to update context key")
	}

	f.log.Debug("Updating Composition Pipeline Context", "key", contextField, "data", results)

	// Convert the updated context back into structpb.Struct
	updatedContext, err := structpb.NewStruct(contextMap)
	if err != nil {
		return errors.Wrap(err, "failed to serialize updated context")
	}

	// Set the updated context
	rsp.Context = updatedContext
	return nil
}

// targetHasData checks if a target field already has data
func targetHasData(data map[string]interface{}, key string) (bool, error) {
	parts, err := ParseNestedKey(key)
	if err != nil {
		return false, err
	}

	currentValue := interface{}(data)
	for _, k := range parts {
		// Check if the current value is a map
		if nestedMap, ok := currentValue.(map[string]interface{}); ok {
			// Get the next value in the nested map
			if nextValue, exists := nestedMap[k]; exists {
				currentValue = nextValue
			} else {
				// Key doesn't exist, so no data
				return false, nil
			}
		} else {
			// Not a map, so can't traverse further
			return false, nil
		}
	}

	// If we've reached here, the key exists
	// Check if it has meaningful data (not nil and not empty)
	if currentValue == nil {
		return false, nil
	}

	// Check for empty maps
	if nestedMap, ok := currentValue.(map[string]interface{}); ok {
		return len(nestedMap) > 0, nil
	}

	// Check for empty slices
	if slice, ok := currentValue.([]interface{}); ok {
		return len(slice) > 0, nil
	}

	// For strings, check if empty
	if str, ok := currentValue.(string); ok {
		return str != "", nil
	}

	// For other types (numbers, booleans), consider them as having data
	return true, nil
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

// isValidTarget checks if the target is valid
func (f *Function) isValidTarget(target string) bool {
	return strings.HasPrefix(target, "status.") || strings.HasPrefix(target, "context.")
}

// shouldSkipQuery checks if the query should be skipped.
func (f *Function) shouldSkipQuery(req *fnv1.RunFunctionRequest, in *v1beta1.Input, rsp *fnv1.RunFunctionResponse) bool {
	// Determine if we should skip the query when target has data
	var shouldSkipQueryWhenTargetHasData = false // Default to false to ensure continuous reconciliation
	if in.SkipQueryWhenTargetHasData != nil {
		shouldSkipQueryWhenTargetHasData = *in.SkipQueryWhenTargetHasData
	}

	if !shouldSkipQueryWhenTargetHasData {
		return false
	}

	switch {
	case strings.HasPrefix(in.Target, "status."):
		return f.checkStatusTargetHasData(req, in, rsp)
	case strings.HasPrefix(in.Target, "context."):
		return f.checkContextTargetHasData(req, in, rsp)
	}

	return false
}

// checkContextTargetHasData checks if the context target has data.
func (f *Function) checkContextTargetHasData(req *fnv1.RunFunctionRequest, in *v1beta1.Input, rsp *fnv1.RunFunctionResponse) bool {
	contextMap := req.GetContext().AsMap()
	contextField := strings.TrimPrefix(in.Target, "context.")
	if hasData, _ := targetHasData(contextMap, contextField); hasData {
		f.log.Info("Target already has data, skipping query", "target", in.Target)

		// Set success condition and return
		response.ConditionTrue(rsp, "FunctionSkip", "SkippedQuery").
			WithMessage("Target already has data, skipped query to avoid throttling").
			TargetCompositeAndClaim()
		return true
	}
	return false
}
