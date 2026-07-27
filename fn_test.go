package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/upbound/function-msgraph/input/v1beta1"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"k8s.io/utils/ptr"

	"github.com/crossplane/function-sdk-go/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

const (
	// Repeated fixture values used across the table-driven tests.
	testRequestTag      = "hello"
	testCredentialsKey  = "credentials"
	testAzureCredsName  = "azure-creds"
	testSPID1           = "sp-id-1"
	testUserID1         = "user-id-1"
	testUserIDDefault   = "test-user-id"
	testUserDisplay     = "Test User"
	testUser1Email      = "user1@example.com"
	testUser2Email      = "user2@example.com"
	testExampleEmail    = "user@example.com"
	testFieldCity       = "city"
	testFieldDepartment = "department"
	watchedResourceKey  = "ops.crossplane.io/watched-resource"

	// Condition fields asserted on successful function responses.
	condTypeFunctionSuccess = "FunctionSuccess"
	condReasonSuccess       = "Success"

	// Result messages emitted per query type.
	msgUserValidationQueryType          = `QueryType: "UserValidation"`
	msgGroupObjectIDsQueryType          = `QueryType: "GroupObjectIDs"`
	msgServicePrincipalDetailsQueryType = `QueryType: "ServicePrincipalDetails"`
)

type MockGraphQuery struct {
	GraphQueryFunc func(ctx context.Context, azureCreds map[string]string, in *v1beta1.Input) (interface{}, error)
}

func (m *MockGraphQuery) graphQuery(ctx context.Context, azureCreds map[string]string, in *v1beta1.Input) (interface{}, error) {
	return m.GraphQueryFunc(ctx, azureCreds, in)
}

type MockTimer struct{}

func (MockTimer) now() string {
	return "2025-01-01T00:00:00+01:00"
}

// TestResolveGroupsRef tests the functionality of resolving groupsRef from context, status, or spec
func TestResolveGroupsRef(t *testing.T) {
	var (
		xr    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
		creds = &fnv1.CredentialData{
			Data: map[string][]byte{
				testCredentialsKey: []byte(`{
"clientId": "test-client-id",
"clientSecret": "test-client-secret",
"subscriptionId": "test-subscription-id",
"tenantId": "test-tenant-id"
}`),
			},
		}
	)

	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"GroupsRefFromStatus": {
			reason: "The Function should resolve groupsRef from XR status",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groupsRef": "status.groups",
						"target": "status.groupObjectIDs"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"groups": ["Developers", "Operations", "All Company"]
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgGroupObjectIDsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"groups": ["Developers", "Operations", "All Company"],
									"groupObjectIDs": [
										{
											"id": "group-id-1",
											"displayName": "Developers",
											"description": "Development team"
										},
										{
											"id": "group-id-2",
											"displayName": "Operations",
											"description": "Operations team"
										},
										{
											"id": "group-id-3",
											"displayName": "All Company",
											"description": "All company group"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"GroupsRefFromContext": {
			reason: "The Function should resolve groupsRef from context",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groupsRef": "context.groups",
						"target": "status.groupObjectIDs"
					}`),
					Context: resource.MustStructJSON(`{
						"groups": ["Developers", "Operations", "All Company"]
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgGroupObjectIDsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Context: resource.MustStructJSON(`{
						"groups": ["Developers", "Operations", "All Company"]
					}`),
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"groupObjectIDs": [
										{
											"id": "group-id-1",
											"displayName": "Developers",
											"description": "Development team"
										},
										{
											"id": "group-id-2",
											"displayName": "Operations",
											"description": "Operations team"
										},
										{
											"id": "group-id-3",
											"displayName": "All Company",
											"description": "All company group"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"GroupsRefFromSpec": {
			reason: "The Function should resolve groupsRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groupsRef": "spec.groupConfig.groupNames",
						"target": "status.groupObjectIDs"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"groupConfig": {
										"groupNames": ["Developers", "Operations", "All Company"]
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgGroupObjectIDsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"groupConfig": {
										"groupNames": ["Developers", "Operations", "All Company"]
									}
								},
								"status": {
									"groupObjectIDs": [
										{
											"id": "group-id-1",
											"displayName": "Developers",
											"description": "Development team"
										},
										{
											"id": "group-id-2",
											"displayName": "Operations",
											"description": "Operations team"
										},
										{
											"id": "group-id-3",
											"displayName": "All Company",
											"description": "All company group"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"GroupsRefEmptyDefault": {
			reason: "The Function should resolve groupsRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groupsRef": "spec.groupConfig.groupNames",
						"target": "status.groupObjectIDs"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"groupConfig": {
										"groupNames": []
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgGroupObjectIDsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"groupConfig": {
										"groupNames": []
									}
								},
								"status": {
									"groupObjectIDs": []
								}}`),
						},
					},
				},
			},
		},
		"GroupsRefEmptyNoFail": {
			reason: "The Function should resolve groupsRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groupsRef": "spec.groupConfig.groupNames",
						"target": "status.groupObjectIDs",
						"failOnEmpty": false
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"groupConfig": {
										"groupNames": []
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgGroupObjectIDsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"groupConfig": {
										"groupNames": []
									}
								},
								"status": {
									"groupObjectIDs": []
								}}`),
						},
					},
				},
			},
		},
		"GroupsRefEmptyFail": {
			reason: "The Function should resolve groupsRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groupsRef": "spec.groupConfig.groupNames",
						"target": "status.groupObjectIDs",
						"failOnEmpty": true
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"groupConfig": {
										"groupNames": []
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:       &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "no group names provided",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"groupConfig": {
										"groupNames": []
									}
								}}`),
						},
					},
				},
			},
		},
		"GroupsRefNotFound": {
			reason: "The Function should handle an error when groupsRef cannot be resolved",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groupsRef": "context.nonexistent.value",
						"target": "status.groupObjectIDs"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot resolve groupsRef: context.nonexistent.value not found",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create mock responders for each type of query
			mockQuery := &MockGraphQuery{
				GraphQueryFunc: func(_ context.Context, _ map[string]string, in *v1beta1.Input) (interface{}, error) {
					if in.QueryType == "GroupObjectIDs" {
						if in.FailOnEmpty != nil && *in.FailOnEmpty && len(in.Groups) == 0 {
							return nil, errors.New("no group names provided")
						}

						results := make([]interface{}, 0)
						for i, group := range in.Groups {
							if group == nil {
								continue
							}

							groupID := fmt.Sprintf("group-id-%d", i+1)
							var description string
							switch *group {
							case "Operations":
								description = "Operations team"
							case "All Company":
								description = "All company group"
							default:
								description = "Development team"
							}

							groupMap := map[string]interface{}{
								"id":             groupID,
								fieldDisplayName: *group,
								fieldDescription: description,
							}
							results = append(results, groupMap)
						}
						return results, nil
					}
					return nil, errors.Errorf("unsupported query type: %s", in.QueryType)
				},
			}

			f := &Function{
				graphQuery: mockQuery,
				log:        logging.NewNopLogger(),
			}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestResolveGroupRef tests the functionality of resolving groupRef from context, status, or spec
func TestResolveGroupRef(t *testing.T) {
	var (
		xr    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
		creds = &fnv1.CredentialData{
			Data: map[string][]byte{
				testCredentialsKey: []byte(`{
"clientId": "test-client-id",
"clientSecret": "test-client-secret",
"subscriptionId": "test-subscription-id",
"tenantId": "test-tenant-id"
}`),
			},
		}
	)

	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"GroupRefFromStatus": {
			reason: "The Function should resolve groupRef from XR status",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupMembership",
						"groupRef": "status.groupInfo.name",
						"target": "status.groupMembers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"groupInfo": {
										"name": "Developers"
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  `QueryType: "GroupMembership"`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"groupInfo": {
										"name": "Developers"
									},
									"groupMembers": [
										{
											"id": "user-id-1",
											"displayName": "Test User 1",
											"mail": "user1@example.com",
											"type": "user",
											"userPrincipalName": "user1@example.com"
										},
										{
											"id": "sp-id-1",
											"displayName": "Test Service Principal",
											"appId": "sp-app-id-1",
											"type": "servicePrincipal"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"GroupRefFromContext": {
			reason: "The Function should resolve groupRef from context",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupMembership",
						"groupRef": "context.groupInfo.name",
						"target": "status.groupMembers"
					}`),
					Context: resource.MustStructJSON(`{
						"groupInfo": {
							"name": "Developers"
						}
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  `QueryType: "GroupMembership"`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Context: resource.MustStructJSON(`{
						"groupInfo": {
							"name": "Developers"
						}
					}`),
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"groupMembers": [
										{
											"id": "user-id-1",
											"displayName": "Test User 1",
											"mail": "user1@example.com",
											"type": "user",
											"userPrincipalName": "user1@example.com"
										},
										{
											"id": "sp-id-1",
											"displayName": "Test Service Principal",
											"appId": "sp-app-id-1",
											"type": "servicePrincipal"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"GroupRefFromSpec": {
			reason: "The Function should resolve groupRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupMembership",
						"groupRef": "spec.groupConfig.name",
						"target": "status.groupMembers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"groupConfig": {
										"name": "Developers"
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  `QueryType: "GroupMembership"`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"groupConfig": {
										"name": "Developers"
									}
								},
								"status": {
									"groupMembers": [
										{
											"id": "user-id-1",
											"displayName": "Test User 1",
											"mail": "user1@example.com",
											"type": "user",
											"userPrincipalName": "user1@example.com"
										},
										{
											"id": "sp-id-1",
											"displayName": "Test Service Principal",
											"appId": "sp-app-id-1",
											"type": "servicePrincipal"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"GroupRefNotFound": {
			reason: "The Function should handle an error when groupRef cannot be resolved",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupMembership",
						"groupRef": "context.nonexistent.value",
						"target": "status.groupMembers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot resolve groupRef: context.nonexistent.value not found",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create mock responders for each type of query
			mockQuery := &MockGraphQuery{
				GraphQueryFunc: func(_ context.Context, _ map[string]string, in *v1beta1.Input) (interface{}, error) {
					if in.QueryType == "GroupMembership" {
						if in.Group == nil || *in.Group == "" {
							return nil, errors.New("no group name provided")
						}
						return []interface{}{
							map[string]interface{}{
								"id":                   testUserID1,
								fieldDisplayName:       "Test User 1",
								fieldMail:              testUser1Email,
								fieldUserPrincipalName: testUser1Email,
								fieldType:              userType,
							},
							map[string]interface{}{
								"id":             testSPID1,
								fieldDisplayName: "Test Service Principal",
								fieldAppID:       "sp-app-id-1",
								fieldType:        servicePrincipalType,
							},
						}, nil
					}
					return nil, errors.Errorf("unsupported query type: %s", in.QueryType)
				},
			}

			f := &Function{
				graphQuery: mockQuery,
				log:        logging.NewNopLogger(),
			}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestResolveUsersRef tests the functionality of resolving usersRef from context, status, or spec
func TestResolveUsersRef(t *testing.T) {
	var (
		xr    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
		creds = &fnv1.CredentialData{
			Data: map[string][]byte{
				testCredentialsKey: []byte(`{
"clientId": "test-client-id",
"clientSecret": "test-client-secret",
"subscriptionId": "test-subscription-id",
"tenantId": "test-tenant-id"
}`),
			},
		}
	)

	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"UsersRefFromStatus": {
			reason: "The Function should resolve usersRef from XR status",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"usersRef": "status.users",
						"target": "status.validatedUsers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"users": ["user1@example.com", "user2@example.com", "admin@example.onmicrosoft.com"]
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"users": ["user1@example.com", "user2@example.com", "admin@example.onmicrosoft.com"],
									"validatedUsers": [
										{
											"id": "user-id-1",
											"displayName": "User 1",
											"userPrincipalName": "user1@example.com",
											"mail": "user1@example.com"
										},
										{
											"id": "user-id-2",
											"displayName": "User 2",
											"userPrincipalName": "user2@example.com",
											"mail": "user2@example.com"
										},
										{
											"id": "admin-id",
											"displayName": "Admin User",
											"userPrincipalName": "admin@example.onmicrosoft.com",
											"mail": "admin@example.onmicrosoft.com"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"UsersRefFromContext": {
			reason: "The Function should resolve usersRef from context",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"usersRef": "context.users",
						"target": "status.validatedUsers"
					}`),
					Context: resource.MustStructJSON(`{
						"users": ["user1@example.com", "user2@example.com", "admin@example.onmicrosoft.com"]
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Context: resource.MustStructJSON(`{
						"users": ["user1@example.com", "user2@example.com", "admin@example.onmicrosoft.com"]
					}`),
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"validatedUsers": [
										{
											"id": "user-id-1",
											"displayName": "User 1",
											"userPrincipalName": "user1@example.com",
											"mail": "user1@example.com"
										},
										{
											"id": "user-id-2",
											"displayName": "User 2",
											"userPrincipalName": "user2@example.com",
											"mail": "user2@example.com"
										},
										{
											"id": "admin-id",
											"displayName": "Admin User",
											"userPrincipalName": "admin@example.onmicrosoft.com",
											"mail": "admin@example.onmicrosoft.com"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"UsersRefFromSpec": {
			reason: "The Function should resolve usersRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"usersRef": "spec.userAccess.emails",
						"target": "status.validatedUsers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"userAccess": {
										"emails": ["user1@example.com", "user2@example.com", "admin@example.onmicrosoft.com"]
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"userAccess": {
										"emails": ["user1@example.com", "user2@example.com", "admin@example.onmicrosoft.com"]
									}
								},
								"status": {
									"validatedUsers": [
										{
											"id": "user-id-1",
											"displayName": "User 1",
											"userPrincipalName": "user1@example.com",
											"mail": "user1@example.com"
										},
										{
											"id": "user-id-2",
											"displayName": "User 2",
											"userPrincipalName": "user2@example.com",
											"mail": "user2@example.com"
										},
										{
											"id": "admin-id",
											"displayName": "Admin User",
											"userPrincipalName": "admin@example.onmicrosoft.com",
											"mail": "admin@example.onmicrosoft.com"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"UsersRefEmptyDefault": {
			reason: "The Function should resolve usersRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"usersRef": "spec.userAccess.emails",
						"target": "status.validatedUsers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"userAccess": {
										"emails": []
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"userAccess": {
										"emails": []
									}
								},
								"status": {
									"validatedUsers": []
								}}`),
						},
					},
				},
			},
		},
		"UsersRefEmptyNoFail": {
			reason: "The Function should resolve usersRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"usersRef": "spec.userAccess.emails",
						"target": "status.validatedUsers",
						"failOnEmpty": false
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"userAccess": {
										"emails": []
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"userAccess": {
										"emails": []
									}
								},
								"status": {
									"validatedUsers": []
								}}`),
						},
					},
				},
			},
		},
		"UsersRefEmptyFail": {
			reason: "The Function should resolve usersRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"usersRef": "spec.userAccess.emails",
						"target": "status.validatedUsers",
						"failOnEmpty": true
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"userAccess": {
										"emails": []
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:       &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "no users provided for validation",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"userAccess": {
										"emails": []
									}
								}}`),
						},
					},
				},
			},
		},
		"UsersRefNotFound": {
			reason: "The Function should handle an error when usersRef cannot be resolved",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"usersRef": "context.nonexistent.value",
						"target": "status.validatedUsers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot resolve usersRef: context.nonexistent.value not found",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create mock responders for each type of query
			mockQuery := &MockGraphQuery{
				GraphQueryFunc: func(_ context.Context, _ map[string]string, in *v1beta1.Input) (interface{}, error) {
					if in.QueryType == "UserValidation" {
						if in.FailOnEmpty != nil && *in.FailOnEmpty && len(in.Users) == 0 {
							return nil, errors.New("no users provided for validation")
						}

						results := make([]interface{}, 0)
						for _, user := range in.Users {
							if user == nil {
								continue
							}

							var (
								userID      string
								displayName string
							)

							// Generate different test data based on user principal name
							switch *user {
							case testUser1Email:
								userID = testUserID1
								displayName = "User 1"
							case testUser2Email:
								userID = "user-id-2"
								displayName = "User 2"
							case "admin@example.onmicrosoft.com":
								userID = "admin-id"
								displayName = "Admin User"
							default:
								userID = testUserIDDefault
								displayName = testUserDisplay
							}

							userMap := map[string]interface{}{
								"id":                   userID,
								fieldDisplayName:       displayName,
								fieldUserPrincipalName: *user,
								fieldMail:              *user,
							}
							results = append(results, userMap)
						}
						return results, nil
					}
					return nil, errors.Errorf("unsupported query type: %s", in.QueryType)
				},
			}

			f := &Function{
				graphQuery: mockQuery,
				log:        logging.NewNopLogger(),
			}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestResolveServicePrincipalsRef tests the functionality of resolving servicePrincipalsRef from context, status, or spec
func TestResolveServicePrincipalsRef(t *testing.T) {
	var (
		xr    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
		creds = &fnv1.CredentialData{
			Data: map[string][]byte{
				testCredentialsKey: []byte(`{
"clientId": "test-client-id",
"clientSecret": "test-client-secret",
"subscriptionId": "test-subscription-id",
"tenantId": "test-tenant-id"
}`),
			},
		}
	)

	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ServicePrincipalsRefFromStatus": {
			reason: "The Function should resolve servicePrincipalsRef from XR status",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "ServicePrincipalDetails",
						"servicePrincipalsRef": "status.servicePrincipalNames",
						"target": "status.servicePrincipals"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"servicePrincipalNames": ["MyServiceApp", "ApiConnector", "yury-upbound-oidc-provider"]
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgServicePrincipalDetailsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"servicePrincipalNames": ["MyServiceApp", "ApiConnector", "yury-upbound-oidc-provider"],
									"servicePrincipals": [
										{
											"id": "sp-id-1",
											"appId": "app-id-1",
											"displayName": "MyServiceApp",
											"description": "Service application"
										},
										{
											"id": "sp-id-2",
											"appId": "app-id-2",
											"displayName": "ApiConnector",
											"description": "API connector application"
										},
										{
											"id": "sp-id-3",
											"appId": "app-id-3",
											"displayName": "yury-upbound-oidc-provider",
											"description": "OIDC provider application"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"ServicePrincipalsRefFromContext": {
			reason: "The Function should resolve servicePrincipalsRef from context",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "ServicePrincipalDetails",
						"servicePrincipalsRef": "context.servicePrincipalNames",
						"target": "status.servicePrincipals"
					}`),
					Context: resource.MustStructJSON(`{
						"servicePrincipalNames": ["MyServiceApp", "ApiConnector", "yury-upbound-oidc-provider"]
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgServicePrincipalDetailsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Context: resource.MustStructJSON(`{
						"servicePrincipalNames": ["MyServiceApp", "ApiConnector", "yury-upbound-oidc-provider"]
					}`),
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"servicePrincipals": [
										{
											"id": "sp-id-1",
											"appId": "app-id-1",
											"displayName": "MyServiceApp",
											"description": "Service application"
										},
										{
											"id": "sp-id-2",
											"appId": "app-id-2",
											"displayName": "ApiConnector",
											"description": "API connector application"
										},
										{
											"id": "sp-id-3",
											"appId": "app-id-3",
											"displayName": "yury-upbound-oidc-provider",
											"description": "OIDC provider application"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"ServicePrincipalsRefFromSpec": {
			reason: "The Function should resolve servicePrincipalsRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "ServicePrincipalDetails",
						"servicePrincipalsRef": "spec.servicePrincipalConfig.names",
						"target": "status.servicePrincipals"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"servicePrincipalConfig": {
										"names": ["MyServiceApp", "ApiConnector", "yury-upbound-oidc-provider"]
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgServicePrincipalDetailsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"servicePrincipalConfig": {
										"names": ["MyServiceApp", "ApiConnector", "yury-upbound-oidc-provider"]
									}
								},
								"status": {
									"servicePrincipals": [
										{
											"id": "sp-id-1",
											"appId": "app-id-1",
											"displayName": "MyServiceApp",
											"description": "Service application"
										},
										{
											"id": "sp-id-2",
											"appId": "app-id-2",
											"displayName": "ApiConnector",
											"description": "API connector application"
										},
										{
											"id": "sp-id-3",
											"appId": "app-id-3",
											"displayName": "yury-upbound-oidc-provider",
											"description": "OIDC provider application"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"ServicePrincipalsRefEmptyDefault": {
			reason: "The Function should resolve servicePrincipalsRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "ServicePrincipalDetails",
						"servicePrincipalsRef": "spec.servicePrincipalConfig.names",
						"target": "status.servicePrincipals"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"servicePrincipalConfig": {
										"names": []
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgServicePrincipalDetailsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"servicePrincipalConfig": {
										"names": []
									}
								},
								"status": {
									"servicePrincipals": []
								}}`),
						},
					},
				},
			},
		},
		"ServicePrincipalsRefEmptyNoFail": {
			reason: "The Function should resolve servicePrincipalsRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "ServicePrincipalDetails",
						"servicePrincipalsRef": "spec.servicePrincipalConfig.names",
						"target": "status.servicePrincipals",
						"failOnEmpty": false
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"servicePrincipalConfig": {
										"names": []
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgServicePrincipalDetailsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"servicePrincipalConfig": {
										"names": []
									}
								},
								"status": {
									"servicePrincipals": []
								}}`),
						},
					},
				},
			},
		},
		"ServicePrincipalsRefEmptyFail": {
			reason: "The Function should resolve servicePrincipalsRef from XR spec",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "ServicePrincipalDetails",
						"servicePrincipalsRef": "spec.servicePrincipalConfig.names",
						"target": "status.servicePrincipals",
						"failOnEmpty": true
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"servicePrincipalConfig": {
										"names": []
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta:       &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "no service principal names provided",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"spec": {
									"servicePrincipalConfig": {
										"names": []
									}
								}}`),
						},
					},
				},
			},
		},
		"ServicePrincipalsRefNotFound": {
			reason: "The Function should handle an error when servicePrincipalsRef cannot be resolved",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "ServicePrincipalDetails",
						"servicePrincipalsRef": "context.nonexistent.value",
						"target": "status.servicePrincipals"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "cannot resolve servicePrincipalsRef: context.nonexistent.value not found",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create mock responders for each type of query
			mockQuery := &MockGraphQuery{
				GraphQueryFunc: func(_ context.Context, _ map[string]string, in *v1beta1.Input) (interface{}, error) {
					if in.QueryType == "ServicePrincipalDetails" {
						if in.FailOnEmpty != nil && *in.FailOnEmpty && len(in.ServicePrincipals) == 0 {
							return nil, errors.New("no service principal names provided")
						}

						results := make([]interface{}, 0)
						for i, sp := range in.ServicePrincipals {
							if sp == nil {
								continue
							}

							var (
								spID        string
								appID       string
								description string
							)

							// Generate different test data based on service principal name
							switch *sp {
							case "MyServiceApp":
								spID = testSPID1
								appID = "app-id-1"
								description = "Service application"
							case "ApiConnector":
								spID = "sp-id-2"
								appID = "app-id-2"
								description = "API connector application"
							case "yury-upbound-oidc-provider":
								spID = "sp-id-3"
								appID = "app-id-3"
								description = "OIDC provider application"
							default:
								spID = fmt.Sprintf("sp-id-%d", i+1)
								appID = fmt.Sprintf("app-id-%d", i+1)
								description = "Generic service principal"
							}

							spMap := map[string]interface{}{
								"id":             spID,
								fieldAppID:       appID,
								fieldDisplayName: *sp,
								fieldDescription: description,
							}
							results = append(results, spMap)
						}
						return results, nil
					}
					return nil, errors.Errorf("unsupported query type: %s", in.QueryType)
				},
			}

			f := &Function{
				graphQuery: mockQuery,
				log:        logging.NewNopLogger(),
			}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRunFunction(t *testing.T) {

	var (
		xr    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr","finalizers":["composite.apiextensions.crossplane.io"]},"spec":{"count":2}}`
		creds = &fnv1.CredentialData{
			Data: map[string][]byte{
				testCredentialsKey: []byte(`{
"clientId": "test-cliend-id",
"clientSecret": "test-client-secret",
"subscriptionId": "test-subscription-id",
"tenantId": "test-tenant-id"
}`),
			},
		}
	)

	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ResponseIsReturned": {
			reason: "The Function should return a fatal result if no credentials were specified",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"]
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "failed to get azure-creds credentials",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
		"MissingUserValidationTarget": {
			reason: "The Function should return a fatal result if no target is specified",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"]
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "Unrecognized target field: ",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
		"UserValidationMissingUsers": {
			reason: "The Function should handle UserValidation with missing users",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"target": "status.validatedUsers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "no users provided for validation",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
		"SuccessfulUserValidation": {
			reason: "The Function should handle a successful UserValidation query",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"validatedUsers": [
										{
											"id": "test-user-id",
											"displayName": "Test User",
											"userPrincipalName": "user@example.com",
											"mail": "user@example.com"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"UserValidationActiveAccount": {
			reason: "The Function should pass activeAccount through to the query and store only the enabled users",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com", "disabled@example.com"],
						"activeAccount": true,
						"target": "status.validatedUsers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"validatedUsers": [
										{
											"id": "test-user-id",
											"displayName": "Test User",
											"userPrincipalName": "user@example.com",
											"mail": "user@example.com",
											"accountEnabled": true
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"GroupMembershipMissingGroup": {
			reason: "The Function should handle GroupMembership with missing group",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupMembership",
						"target": "status.groupMembers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "no group name provided",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
		"SuccessfulGroupMembership": {
			reason: "The Function should handle a successful GroupMembership query",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupMembership",
						"group": "Developers",
						"target": "status.groupMembers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  `QueryType: "GroupMembership"`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"groupMembers": [
										{
											"id": "user-id-1",
											"displayName": "Test User 1",
											"mail": "user1@example.com",
											"type": "user",
											"userPrincipalName": "user1@example.com"
										},
										{
											"id": "sp-id-1",
											"displayName": "Test Service Principal",
											"appId": "sp-app-id-1",
											"type": "servicePrincipal"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"GroupObjectIDsMissingGroups": {
			reason: "The Function should handle GroupObjectIDs with missing groups",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"target": "status.groupObjectIDs"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "no group names provided",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
		"SuccessfulGroupObjectIDs": {
			reason: "The Function should handle a successful GroupObjectIDs query",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groups": ["Developers", "Operations"],
						"target": "status.groupObjectIDs"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgGroupObjectIDsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"groupObjectIDs": [
										{
											"id": "group-id-1",
											"displayName": "Developers",
											"description": "Development team"
										},
										{
											"id": "group-id-2",
											"displayName": "Operations",
											"description": "Operations team"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"ServicePrincipalDetailsMissingNames": {
			reason: "The Function should handle ServicePrincipalDetails with missing names",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "ServicePrincipalDetails",
						"target": "status.servicePrincipals"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "no service principal names provided",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
		"SuccessfulServicePrincipalDetails": {
			reason: "The Function should handle a successful ServicePrincipalDetails query",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "ServicePrincipalDetails",
						"servicePrincipals": ["MyServiceApp"],
						"target": "status.servicePrincipals"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgServicePrincipalDetailsQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"servicePrincipals": [
										{
											"id": "sp-id-1",
											"appId": "app-id-1",
											"displayName": "MyServiceApp",
											"description": "Service application"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"InvalidQueryType": {
			reason: "The Function should handle an invalid query type",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "InvalidType",
						"target": "status.invalidResult"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "unsupported query type: InvalidType",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
		"ShouldSkipQueryWhenStatusTargetHasData": {
			reason: "The Function should skip query when status target already has data",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers",
						"skipQueryWhenTargetHasData": true
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"validatedUsers": [
										{
											"id": "existing-user-id",
											"displayName": "Existing User",
											"userPrincipalName": "existing@example.com",
											"mail": "existing@example.com"
										}
									]
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:    "FunctionSkip",
							Message: ptr.To("Target already has data, skipped query to avoid throttling"),
							Status:  fnv1.Status_STATUS_CONDITION_TRUE,
							Reason:  "SkippedQuery",
							Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"validatedUsers": [
										{
											"id": "existing-user-id",
											"displayName": "Existing User",
											"userPrincipalName": "existing@example.com",
											"mail": "existing@example.com"
										}
									]
								}}`),
						},
					},
				},
			},
		},
		"SkipQueryDueToInterval": {
			reason: "The Function should skip the query when the queryInterval has not elapsed since the last query",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers",
						"queryInterval": "10m"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"validatedUsers": [
										{
											"id": "existing-user-id",
											"displayName": "Existing User",
											"userPrincipalName": "existing@example.com",
											"mail": "existing@example.com"
										}
									],
									"lastQueryTimestamps": {
										"validatedUsers": "2024-12-31T23:55:00+01:00"
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:    "FunctionSkip",
							Message: ptr.To("Query skipped due to interval limit (10m)"),
							Status:  fnv1.Status_STATUS_CONDITION_TRUE,
							Reason:  "IntervalLimit",
							Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"validatedUsers": [
										{
											"id": "existing-user-id",
											"displayName": "Existing User",
											"userPrincipalName": "existing@example.com",
											"mail": "existing@example.com"
										}
									],
									"lastQueryTimestamps": {
										"validatedUsers": "2024-12-31T23:55:00+01:00"
									}
								}}`),
						},
					},
				},
			},
		},
		"RunQueryWhenIntervalElapsed": {
			reason: "The Function should run the query and refresh the timestamp when the queryInterval has elapsed",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers",
						"queryInterval": "10m"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"validatedUsers": [
										{
											"id": "existing-user-id",
											"displayName": "Existing User",
											"userPrincipalName": "existing@example.com",
											"mail": "existing@example.com"
										}
									],
									"lastQueryTimestamps": {
										"validatedUsers": "2024-12-31T22:00:00+01:00"
									}
								}
							}`),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"status": {
									"validatedUsers": [
										{
											"id": "test-user-id",
											"displayName": "Test User",
											"userPrincipalName": "user@example.com",
											"mail": "user@example.com"
										}
									],
									"lastQueryTimestamps": {
										"validatedUsers": "2025-01-01T00:00:00+01:00"
									}
								}}`),
						},
					},
				},
			},
		},
		"RunQueryFirstTimeWithInterval": {
			reason: "The Function should run the query and record the query timestamp on the first run when queryInterval is set and no prior data exists",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers",
						"queryInterval": "10m"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"validatedUsers": [
										{
											"id": "test-user-id",
											"displayName": "Test User",
											"userPrincipalName": "user@example.com",
											"mail": "user@example.com"
										}
									],
									"lastQueryTimestamps": {
										"validatedUsers": "2025-01-01T00:00:00+01:00"
									}
								}}`),
						},
					},
				},
			},
		},
		"InvalidQueryInterval": {
			reason: "The Function should return a fatal result when queryInterval cannot be parsed as a Go duration",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers",
						"queryInterval": "notaduration"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  `cannot parse queryInterval "notaduration" as a Go duration (e.g. "10m"): time: invalid duration "notaduration"`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
		"WarnWhenIntervalAndSkipWhenTargetHasDataCombined": {
			reason: "The Function should emit a non-fatal warning (and still run) when queryInterval and skipQueryWhenTargetHasData are both set",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers",
						"queryInterval": "10m",
						"skipQueryWhenTargetHasData": true
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_WARNING,
							Message:  "both queryInterval and skipQueryWhenTargetHasData are set; skipQueryWhenTargetHasData takes precedence once the target has data, so the queryInterval refresh will not run",
							Target:   fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								},
								"status": {
									"validatedUsers": [
										{
											"id": "test-user-id",
											"displayName": "Test User",
											"userPrincipalName": "user@example.com",
											"mail": "user@example.com"
										}
									],
									"lastQueryTimestamps": {
										"validatedUsers": "2025-01-01T00:00:00+01:00"
									}
								}}`),
						},
					},
				},
			},
		},
		"QueryToContextField": {
			reason: "The Function should store results in context field",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "context.validatedUsers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Context: resource.MustStructJSON(
						`{
							"validatedUsers": [
								{
									"id": "test-user-id",
									"displayName": "Test User",
									"userPrincipalName": "user@example.com",
									"mail": "user@example.com"
								}
							]
						}`,
					),
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.org/v1",
								"kind": "XR",
								"metadata": {
									"name": "cool-xr"
								},
								"spec": {
									"count": 2
								}
							}`),
						},
					},
				},
			},
		},
		"OperationWithoutWatchedResource": {
			reason: "The Function should return fatal if it runs as operation without a watched resource",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "context.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  `operation: no resource to process with name ops.crossplane.io/watched-resource`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"OperationWithLessThanOneWatchedResource": {
			reason: "The Function should return fatal if it runs as operation with less than one watched resource",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "context.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						watchedResourceKey: {
							Items: nil,
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  `operation: incorrect number of resources sent to the function. expected 1, got 0`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"OperationWithMoreThanOneWatchedResource": {
			reason: "The Function should return fatal if it runs as operation with more than one watched resource",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "context.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						watchedResourceKey: {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(xr),
								},
								{
									Resource: resource.MustStructJSON(xr),
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  `operation: incorrect number of resources sent to the function. expected 1, got 2`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"OperationWithNilObjectInWatchedResource": {
			reason: "The Function should return fatal if it runs as operation watched resource with zero length Resource.Object",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "context.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						watchedResourceKey: {
							Items: []*fnv1.Resource{
								{},
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  `operation: Resource.Object property in operation resource can not be empty`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"OperationWithWatchedResourceWhichIsNotXR": {
			reason: "The Function should only allow operations on XRs based on finalizers",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						watchedResourceKey: {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`),
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  "operation: function-msgraph support only operations on composite resources",
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
				},
			},
		},
		"OperationWithWatchedResourceQueryNoDrift": {
			reason: "The Function should set annotations on XR that notify user about lack of drift in query results",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						watchedResourceKey: {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
										"apiVersion": "example.org/v1",
										"kind": "XR",
										"metadata": {
											"name": "cool-xr",
											"finalizers": [
												"composite.apiextensions.crossplane.io"
											]
										},
										"spec": {
											"count": 2
										},
										"status": {
											"validatedUsers": [
												{
													"id": "test-user-id",
													"displayName": "Test User",
													"userPrincipalName": "user@example.com",
													"mail": "user@example.com"
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"xr": {
								Resource: resource.MustStructJSON(`{
									"apiVersion": "example.org/v1",
									"kind": "XR",
									"metadata": {
										"name": "cool-xr",
										"annotations": {
											"function-msgraph/last-execution": "2025-01-01T00:00:00+01:00",
											"function-msgraph/last-execution-query-drift-detected": "false"
										}
									}
								}`),
							},
						},
					},
				},
			},
		},
		"OperationWithWatchedResourceQueryNoDriftWithExistingAnnotations": {
			reason: "The Function should set annotations on XR that notify user about lack of drift in query results and in the same time not override existing annotations",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						watchedResourceKey: {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
										"apiVersion": "example.org/v1",
										"kind": "XR",
										"metadata": {
											"name": "cool-xr",
											"finalizers": [
												"composite.apiextensions.crossplane.io"
											],
											"annotations": {
												"my-cool-annotation": "love-msgraph"
											}
										},
										"spec": {
											"count": 2
										},
										"status": {
											"validatedUsers": [
												{
													"id": "test-user-id",
													"displayName": "Test User",
													"userPrincipalName": "user@example.com",
													"mail": "user@example.com"
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"xr": {
								Resource: resource.MustStructJSON(`{
									"apiVersion": "example.org/v1",
									"kind": "XR",
									"metadata": {
										"name": "cool-xr",
										"annotations": {
											"function-msgraph/last-execution": "2025-01-01T00:00:00+01:00",
											"function-msgraph/last-execution-query-drift-detected": "false",
											"my-cool-annotation": "love-msgraph"
										}
									}
								}`),
							},
						},
					},
				},
			},
		},
		"OperationWithWatchedResourceQueryDrift": {
			reason: "The Function should set annotations on XR that notify user about drift in query results",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						watchedResourceKey: {
							Items: []*fnv1.Resource{
								{
									Resource: resource.MustStructJSON(`{
										"apiVersion": "example.org/v1",
										"kind": "XR",
										"metadata": {
											"name": "cool-xr",
											"finalizers": [
												"composite.apiextensions.crossplane.io"
											]
										},
										"spec": {
											"count": 2
										},
										"status": {
											"validatedUsers": [
												{
													"id": "incorrect-id",
													"displayName": "Another Display Name",
													"userPrincipalName": "user@example.com",
													"mail": "user@example.com"
												}
											]
										}
									}`),
								},
							},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   condTypeFunctionSuccess,
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: condReasonSuccess,
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  msgUserValidationQueryType,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Resources: map[string]*fnv1.Resource{
							"xr": {
								Resource: resource.MustStructJSON(`{
									"apiVersion": "example.org/v1",
									"kind": "XR",
									"metadata": {
										"name": "cool-xr",
										"annotations": {
											"function-msgraph/last-execution": "2025-01-01T00:00:00+01:00",
											"function-msgraph/last-execution-query-drift-detected": "true"
										}
									}
								}`),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create mock responders for each type of query
			mockQuery := &MockGraphQuery{
				GraphQueryFunc: func(_ context.Context, _ map[string]string, in *v1beta1.Input) (interface{}, error) {
					switch in.QueryType {
					case "UserValidation":
						if len(in.Users) == 0 {
							return nil, errors.New("no users provided for validation")
						}
						if ptr.Deref(in.ActiveAccount, false) {
							// Mirrors buildUserResults: the disabled user is dropped
							// and the remaining one carries accountEnabled.
							return []interface{}{
								map[string]interface{}{
									"id":                   testUserIDDefault,
									fieldDisplayName:       testUserDisplay,
									fieldUserPrincipalName: "user@example.com",
									fieldMail:              "user@example.com",
									fieldAccountEnabled:    true,
								},
							}, nil
						}
						return []interface{}{
							map[string]interface{}{
								"id":                   testUserIDDefault,
								fieldDisplayName:       testUserDisplay,
								fieldUserPrincipalName: "user@example.com",
								fieldMail:              "user@example.com",
							},
						}, nil
					case "GroupMembership":
						if in.Group == nil || *in.Group == "" {
							return nil, errors.New("no group name provided")
						}
						return []interface{}{
							map[string]interface{}{
								"id":                   testUserID1,
								fieldDisplayName:       "Test User 1",
								fieldMail:              testUser1Email,
								fieldUserPrincipalName: testUser1Email,
								fieldType:              userType,
							},
							map[string]interface{}{
								"id":             testSPID1,
								fieldDisplayName: "Test Service Principal",
								fieldAppID:       "sp-app-id-1",
								fieldType:        servicePrincipalType,
							},
						}, nil
					case "GroupObjectIDs":
						if len(in.Groups) == 0 {
							return nil, errors.New("no group names provided")
						}
						return []interface{}{
							map[string]interface{}{
								"id":             "group-id-1",
								fieldDisplayName: "Developers",
								fieldDescription: "Development team",
							},
							map[string]interface{}{
								"id":             "group-id-2",
								fieldDisplayName: "Operations",
								fieldDescription: "Operations team",
							},
						}, nil
					case "ServicePrincipalDetails":
						if len(in.ServicePrincipals) == 0 {
							return nil, errors.New("no service principal names provided")
						}
						return []interface{}{
							map[string]interface{}{
								"id":             testSPID1,
								fieldAppID:       "app-id-1",
								fieldDisplayName: "MyServiceApp",
								fieldDescription: "Service application",
							},
						}, nil
					default:
						return nil, errors.Errorf("unsupported query type: %s", in.QueryType)
					}
				},
			}

			f := &Function{
				graphQuery: mockQuery,
				timer:      &MockTimer{},
				log:        logging.NewNopLogger(),
			}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIdentityType(t *testing.T) {
	var (
		xr = `{
				"apiVersion": "example.org/v1",
				"kind": "XR",
				"status": {
					"groups": ["Developers", "Operations", "All Company"]
				}}`
		servicePrincipalCreds = &fnv1.CredentialData{
			Data: map[string][]byte{
				testCredentialsKey: []byte(`{
"clientId": "test-client-id",
"clientSecret": "test-client-secret",
"subscriptionId": "test-subscription-id",
"tenantId": "test-tenant-id"
}`),
			},
		}
		workloadIdentityCredentials = &fnv1.CredentialData{
			Data: map[string][]byte{
				testCredentialsKey: []byte(`{
"federatedTokenFile": "/var/run/secrets/azure/tokens/azure-identity-token"
}`),
			},
		}
	)

	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AzureServicePrincipalCredentialsImplicit": {
			reason: "The Function should default to identity.type AzureServicePrincipalCredentials",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groupsRef": "status.groups",
						"target": "status.groupObjectIDs"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: servicePrincipalCreds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  `failed to initialize service principal provider: failed to obtain clientsecret credentials`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
		"AzureServicePrincipalCredentialsExplicit": {
			reason: "The Function should use ServicePrincipal credentials if identity.type is AzureServicePrincipalCredentials",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groupsRef": "status.groups",
						"target": "status.groupObjectIDs",
						"identity": {
							"type": "AzureServicePrincipalCredentials"
						}
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: servicePrincipalCreds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  `failed to initialize service principal provider: failed to obtain clientsecret credentials`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
		"AzureWorkloadIdentityCredentials": {
			reason: "The Function should use Workload Identity credentials if identity.type is AzureWorkloadIdentityCredentials",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: testRequestTag},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "GroupObjectIDs",
						"groupsRef": "status.groups",
						"target": "status.groupObjectIDs",
						"identity": {
							"type": "AzureWorkloadIdentityCredentials"
						}
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						testAzureCredsName: {
							Source: &fnv1.Credentials_CredentialData{CredentialData: workloadIdentityCredentials},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: testRequestTag, Ttl: durationpb.New(response.DefaultTTL)},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_FATAL,
							Message:  `failed to initialize workload identity provider: failed to obtain workloadidentity credentials`,
							Target:   fnv1.Target_TARGET_COMPOSITE.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create mock responders for each type of query
			mockQuery := &MockGraphQuery{
				GraphQueryFunc: func(_ context.Context, _ map[string]string, in *v1beta1.Input) (interface{}, error) {
					identityType := v1beta1.IdentityTypeAzureServicePrincipalCredentials

					if in.Identity != nil && in.Identity.Type != "" {
						identityType = in.Identity.Type
					}

					switch identityType {
					case v1beta1.IdentityTypeAzureWorkloadIdentityCredentials:
						return nil, errors.New("failed to initialize workload identity provider: failed to obtain workloadidentity credentials")
					case v1beta1.IdentityTypeAzureServicePrincipalCredentials:
						return nil, errors.New("failed to initialize service principal provider: failed to obtain clientsecret credentials")
					default:
						return nil, errors.Errorf("unsupported identity.type: %s", string(identityType))
					}
				},
			}

			f := &Function{
				graphQuery: mockQuery,
				timer:      &MockTimer{},
				log:        logging.NewNopLogger(),
			}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestAdditionalFields verifies that AdditionalFields is propagated to the graph
// query and that extra fields appear in the RunFunction response for every
// supported queryType (UserValidation, GroupObjectIDs, ServicePrincipalDetails,
// GroupMembership).
func TestAdditionalFields(t *testing.T) {
	xr := `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
	creds := &fnv1.CredentialData{
		Data: map[string][]byte{
			testCredentialsKey: []byte(`{
"clientId": "test-client-id",
"clientSecret": "test-client-secret",
"subscriptionId": "test-subscription-id",
"tenantId": "test-tenant-id"
}`),
		},
	}

	cases := map[string]struct {
		reason            string
		inputJSON         string
		wantExtraFields   []string
		wantStatusPayload string
		// mockResult is the value the mock graphQuery returns for this case
		mockResult func(in *v1beta1.Input) (interface{}, error)
	}{
		// ── UserValidation ────────────────────────────────────────────────────────
		"UserValidationWithAdditionalFields": {
			reason: "additionalFields must be forwarded and appear in UserValidation results",
			inputJSON: `{
				"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
				"kind": "Input",
				"queryType": "UserValidation",
				"users": ["user@example.com"],
				"target": "status.validatedUsers",
				"additionalFields": ["city", "department"]
			}`,
			wantExtraFields: []string{testFieldCity, testFieldDepartment},
			wantStatusPayload: `{"validatedUsers":[{
				"id":"test-user-id","displayName":"Test User",
				"userPrincipalName":"user@example.com","mail":"user@example.com",
				"city":"Kyiv","department":"Tech Ops"
			}]}`,
			mockResult: func(in *v1beta1.Input) (interface{}, error) {
				m := map[string]interface{}{
					"id": testUserIDDefault, fieldDisplayName: testUserDisplay,
					fieldUserPrincipalName: testExampleEmail, fieldMail: testExampleEmail,
				}
				for _, f := range in.AdditionalFields {
					switch f {
					case testFieldCity:
						m[testFieldCity] = "Kyiv"
					case testFieldDepartment:
						m[testFieldDepartment] = "Tech Ops"
					}
				}
				return []interface{}{m}, nil
			},
		},
		"UserValidationBackwardCompat": {
			reason: "omitting additionalFields must yield only the default four fields",
			inputJSON: `{
				"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
				"kind": "Input",
				"queryType": "UserValidation",
				"users": ["user@example.com"],
				"target": "status.validatedUsers"
			}`,
			wantExtraFields: nil,
			wantStatusPayload: `{"validatedUsers":[{
				"id":"test-user-id","displayName":"Test User",
				"userPrincipalName":"user@example.com","mail":"user@example.com"
			}]}`,
			mockResult: func(_ *v1beta1.Input) (interface{}, error) {
				return []interface{}{map[string]interface{}{
					"id": testUserIDDefault, fieldDisplayName: testUserDisplay,
					fieldUserPrincipalName: testExampleEmail, fieldMail: testExampleEmail,
				}}, nil
			},
		},
		// ── GroupObjectIDs ───────────────────────────────────────────────────────
		"GroupObjectIDsWithAdditionalFields": {
			reason: "additionalFields must be forwarded and appear in GroupObjectIDs results",
			inputJSON: `{
				"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
				"kind": "Input",
				"queryType": "GroupObjectIDs",
				"groups": ["Developers"],
				"target": "status.groupObjectIDs",
				"additionalFields": ["mailNickname", "visibility"]
			}`,
			wantExtraFields: []string{"mailNickname", "visibility"},
			wantStatusPayload: `{"groupObjectIDs":[{
				"id":"group-id-1","displayName":"Developers","description":"Dev team",
				"mailNickname":"devs","visibility":"Private"
			}]}`,
			mockResult: func(in *v1beta1.Input) (interface{}, error) {
				m := map[string]interface{}{
					"id": "group-id-1", fieldDisplayName: "Developers", fieldDescription: "Dev team",
				}
				for _, f := range in.AdditionalFields {
					switch f {
					case "mailNickname":
						m["mailNickname"] = "devs"
					case "visibility":
						m["visibility"] = "Private"
					}
				}
				return []interface{}{m}, nil
			},
		},
		// ── ServicePrincipalDetails ──────────────────────────────────────────────
		"ServicePrincipalDetailsWithAdditionalFields": {
			reason: "additionalFields must be forwarded and appear in ServicePrincipalDetails results",
			inputJSON: `{
				"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
				"kind": "Input",
				"queryType": "ServicePrincipalDetails",
				"servicePrincipals": ["MyApp"],
				"target": "status.spDetails",
				"additionalFields": ["servicePrincipalType", "homepage"]
			}`,
			wantExtraFields: []string{"servicePrincipalType", "homepage"},
			wantStatusPayload: `{"spDetails":[{
				"id":"sp-id-1","appId":"app-id-1","displayName":"MyApp","description":"",
				"servicePrincipalType":"Application","homepage":"https://myapp.example.com"
			}]}`,
			mockResult: func(in *v1beta1.Input) (interface{}, error) {
				m := map[string]interface{}{
					"id": "sp-id-1", fieldAppID: "app-id-1",
					fieldDisplayName: "MyApp", fieldDescription: "",
				}
				for _, f := range in.AdditionalFields {
					switch f {
					case "servicePrincipalType":
						m["servicePrincipalType"] = "Application"
					case "homepage":
						m["homepage"] = "https://myapp.example.com"
					}
				}
				return []interface{}{m}, nil
			},
		},
		// ── GroupMembership ──────────────────────────────────────────────────────
		"GroupMembershipWithAdditionalFields": {
			reason: "additionalFields must be forwarded and appear in GroupMembership member results",
			inputJSON: `{
				"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
				"kind": "Input",
				"queryType": "GroupMembership",
				"group": "Developers",
				"target": "status.members",
				"additionalFields": ["department", "jobTitle"]
			}`,
			wantExtraFields: []string{testFieldDepartment, "jobTitle"},
			wantStatusPayload: `{"members":[{
				"id":"user-id-1","displayName":"Test User","type":"user",
				"mail":"user1@example.com","userPrincipalName":"user1@example.com",
				"department":"Engineering","jobTitle":"Staff Engineer"
			}]}`,
			mockResult: func(in *v1beta1.Input) (interface{}, error) {
				m := map[string]interface{}{
					"id": testUserID1, fieldDisplayName: testUserDisplay, fieldType: userType,
					fieldMail: testUser1Email, fieldUserPrincipalName: testUser1Email,
				}
				for _, f := range in.AdditionalFields {
					switch f {
					case testFieldDepartment:
						m[testFieldDepartment] = "Engineering"
					case "jobTitle":
						m["jobTitle"] = "Staff Engineer"
					}
				}
				return []interface{}{m}, nil
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var capturedAdditionalFields []string

			mockQuery := &MockGraphQuery{
				GraphQueryFunc: func(_ context.Context, _ map[string]string, in *v1beta1.Input) (interface{}, error) {
					capturedAdditionalFields = in.AdditionalFields
					return tc.mockResult(in)
				},
			}

			f := &Function{
				graphQuery: mockQuery,
				timer:      &MockTimer{},
				log:        logging.NewNopLogger(),
			}

			req := &fnv1.RunFunctionRequest{
				Meta:  &fnv1.RequestMeta{Tag: testRequestTag},
				Input: resource.MustStructJSON(tc.inputJSON),
				Observed: &fnv1.State{
					Composite: &fnv1.Resource{
						Resource: resource.MustStructJSON(xr),
					},
				},
				Credentials: map[string]*fnv1.Credentials{
					testAzureCredsName: {
						Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
					},
				},
			}

			rsp, err := f.RunFunction(context.Background(), req)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", tc.reason, err)
			}

			// AdditionalFields must be forwarded to the mock as-is.
			if diff := cmp.Diff(tc.wantExtraFields, capturedAdditionalFields, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s\nAdditionalFields forwarded to query: -want +got:\n%s", tc.reason, diff)
			}

			// FunctionSuccess condition must be true.
			var successCond *fnv1.Condition
			for _, c := range rsp.GetConditions() {
				if c.GetType() == condTypeFunctionSuccess {
					successCond = c
					break
				}
			}
			if successCond == nil || successCond.GetStatus() != fnv1.Status_STATUS_CONDITION_TRUE {
				t.Errorf("%s: expected FunctionSuccess=True condition, got: %v", tc.reason, rsp.GetConditions())
			}

			// Desired XR status must match expected payload.
			wantDesired := resource.MustStructJSON(`{
				"apiVersion": "example.org/v1",
				"kind": "XR",
				"metadata": {"name": "cool-xr"},
				"spec": {"count": 2},
				"status": ` + tc.wantStatusPayload + `
			}`)
			gotDesired := rsp.GetDesired().GetComposite().GetResource()
			if diff := cmp.Diff(wantDesired, gotDesired, protocmp.Transform()); diff != "" {
				t.Errorf("%s\ndesired XR: -want +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// newTestUser builds a typed user directory object, mirroring how the Graph SDK
// deserializes expanded group members (the values land on the typed struct, not
// in additionalData).
func newTestUser() models.DirectoryObjectable {
	user := models.NewUser()
	user.SetId(ptr.To(testUserID1))
	user.SetDisplayName(ptr.To("Test User 1"))
	user.SetMail(ptr.To(testUser1Email))
	user.SetUserPrincipalName(ptr.To(testUser1Email))
	return user
}

// newTestUserWithoutMail builds a typed user directory object that has no mail
// attribute set (a common Entra ID state where the account has a UPN but the
// mail attribute is null).
func newTestUserWithoutMail() models.DirectoryObjectable {
	user := models.NewUser()
	user.SetId(ptr.To("user-id-3"))
	user.SetDisplayName(ptr.To("No Mail User"))
	user.SetUserPrincipalName(ptr.To("nomail@example.com"))
	// mail intentionally left unset.
	return user
}

// newTestServicePrincipal builds a typed service principal directory object.
func newTestServicePrincipal() models.DirectoryObjectable {
	sp := models.NewServicePrincipal()
	sp.SetId(ptr.To(testSPID1))
	sp.SetDisplayName(ptr.To("Test Service Principal"))
	sp.SetAppId(ptr.To("sp-app-id-1"))
	return sp
}

// newTestDirectoryObject builds a plain directory object whose properties are
// only available via additionalData, exercising the fallback extraction path.
func newTestDirectoryObject() models.DirectoryObjectable {
	do := models.NewDirectoryObject()
	do.SetId(ptr.To("user-id-2"))
	do.SetAdditionalData(map[string]interface{}{
		fieldDisplayName:       "Fallback User",
		fieldMail:              testUser2Email,
		fieldUserPrincipalName: testUser2Email,
	})
	return do
}

// newTestGraphUser builds a typed Graph user as returned by a UserValidation
// lookup, with the accountEnabled attribute set to the given value.
func newTestGraphUser(id, upn string, accountEnabled bool) models.Userable {
	user := models.NewUser()
	user.SetId(ptr.To(id))
	user.SetDisplayName(ptr.To(testUserDisplay))
	user.SetMail(ptr.To(upn))
	user.SetUserPrincipalName(ptr.To(upn))
	user.SetAccountEnabled(ptr.To(accountEnabled))
	return user
}

// newTestGraphUserWithoutAccountEnabled builds a typed Graph user whose
// accountEnabled attribute is absent, as happens when the property is not
// projected via $select or is withheld from the caller.
func newTestGraphUserWithoutAccountEnabled() models.Userable {
	user := models.NewUser()
	user.SetId(ptr.To("user-id-unknown"))
	user.SetDisplayName(ptr.To(testUserDisplay))
	user.SetMail(ptr.To("unknown@example.com"))
	user.SetUserPrincipalName(ptr.To("unknown@example.com"))
	// accountEnabled intentionally left unset.
	return user
}

// newTestGraphUserMinimal builds a typed Graph user carrying only an id and an
// accountEnabled attribute, exercising the empty-string defaults.
func newTestGraphUserMinimal() models.Userable {
	user := models.NewUser()
	user.SetId(ptr.To("user-id-minimal"))
	user.SetAccountEnabled(ptr.To(true))
	return user
}

// TestBuildUserResults exercises the real UserValidation result-building path
// (bypassed by the mocked graphQuery in the RunFunction tests), including the
// accountEnabled filtering enabled by the activeAccount input.
func TestBuildUserResults(t *testing.T) {
	cases := map[string]struct {
		reason               string
		graphUsers           []models.Userable
		requireActiveAccount bool
		additionalFields     []string
		want                 []interface{}
	}{
		"FlagOffKeepsDisabledUsers": {
			reason: "Without activeAccount a disabled user is kept and reported as accountEnabled false",
			graphUsers: []models.Userable{
				newTestGraphUser(testUserID1, testUser1Email, true),
				newTestGraphUser("user-id-2", testUser2Email, false),
			},
			requireActiveAccount: false,
			want: []interface{}{
				map[string]interface{}{
					"id":                   testUserID1,
					fieldDisplayName:       testUserDisplay,
					fieldUserPrincipalName: testUser1Email,
					fieldMail:              testUser1Email,
					fieldAccountEnabled:    true,
				},
				map[string]interface{}{
					"id":                   "user-id-2",
					fieldDisplayName:       testUserDisplay,
					fieldUserPrincipalName: testUser2Email,
					fieldMail:              testUser2Email,
					fieldAccountEnabled:    false,
				},
			},
		},
		"FlagOnDropsDisabledUsers": {
			reason: "With activeAccount only the enabled user is returned",
			graphUsers: []models.Userable{
				newTestGraphUser(testUserID1, testUser1Email, true),
				newTestGraphUser("user-id-2", testUser2Email, false),
			},
			requireActiveAccount: true,
			want: []interface{}{
				map[string]interface{}{
					"id":                   testUserID1,
					fieldDisplayName:       testUserDisplay,
					fieldUserPrincipalName: testUser1Email,
					fieldMail:              testUser1Email,
					fieldAccountEnabled:    true,
				},
			},
		},
		"FlagOnNilAccountEnabledExcluded": {
			reason:               "With activeAccount an unconfirmed account state must fail closed and be excluded",
			graphUsers:           []models.Userable{newTestGraphUserWithoutAccountEnabled()},
			requireActiveAccount: true,
			want:                 []interface{}{},
		},
		"FlagOffNilAccountEnabledReportedFalse": {
			reason:               "Without activeAccount an absent accountEnabled attribute is reported as false",
			graphUsers:           []models.Userable{newTestGraphUserWithoutAccountEnabled()},
			requireActiveAccount: false,
			want: []interface{}{
				map[string]interface{}{
					"id":                   "user-id-unknown",
					fieldDisplayName:       testUserDisplay,
					fieldUserPrincipalName: "unknown@example.com",
					fieldMail:              "unknown@example.com",
					fieldAccountEnabled:    false,
				},
			},
		},
		"EmptyInputReturnsEmptySlice": {
			reason:               "A lookup that matched no user returns an empty list rather than nil",
			graphUsers:           nil,
			requireActiveAccount: true,
			want:                 []interface{}{},
		},
		"NilUserSkipped": {
			reason:               "A nil entry in the returned collection is skipped",
			graphUsers:           []models.Userable{nil, newTestGraphUser(testUserID1, testUser1Email, true)},
			requireActiveAccount: true,
			want: []interface{}{
				map[string]interface{}{
					"id":                   testUserID1,
					fieldDisplayName:       testUserDisplay,
					fieldUserPrincipalName: testUser1Email,
					fieldMail:              testUser1Email,
					fieldAccountEnabled:    true,
				},
			},
		},
		"MissingOptionalFieldsDefaultToEmptyString": {
			reason:               "Absent displayName and mail attributes are reported as empty strings",
			graphUsers:           []models.Userable{newTestGraphUserMinimal()},
			requireActiveAccount: true,
			want: []interface{}{
				map[string]interface{}{
					"id":                   "user-id-minimal",
					fieldDisplayName:       "",
					fieldUserPrincipalName: "",
					fieldMail:              "",
					fieldAccountEnabled:    true,
				},
			},
		},
		"AdditionalFieldsTypedScalar": {
			reason: "A typed scalar additional field (city) is extracted and returned as a string",
			graphUsers: func() []models.Userable {
				u := models.NewUser()
				u.SetId(ptr.To(testUserID1))
				u.SetUserPrincipalName(ptr.To(testUser1Email))
				u.SetMail(ptr.To(testUser1Email))
				u.SetDisplayName(ptr.To(testUserDisplay))
				u.SetAccountEnabled(ptr.To(true))
				u.SetCity(ptr.To("Kyiv"))
				return []models.Userable{u}
			}(),
			requireActiveAccount: false,
			additionalFields:     []string{testFieldCity},
			want: []interface{}{
				map[string]interface{}{
					"id":                   testUserID1,
					fieldDisplayName:       testUserDisplay,
					fieldUserPrincipalName: testUser1Email,
					fieldMail:              testUser1Email,
					fieldAccountEnabled:    true,
					testFieldCity:          "Kyiv",
				},
			},
		},
		"AdditionalFieldsNonScalarSlice": {
			reason: "A []string field (businessPhones) is normalised to []interface{} so it can be serialised",
			graphUsers: func() []models.Userable {
				u := models.NewUser()
				u.SetId(ptr.To(testUserID1))
				u.SetUserPrincipalName(ptr.To(testUser1Email))
				u.SetMail(ptr.To(testUser1Email))
				u.SetDisplayName(ptr.To(testUserDisplay))
				u.SetAccountEnabled(ptr.To(true))
				u.SetBusinessPhones([]string{"+1-555-0100"})
				return []models.Userable{u}
			}(),
			requireActiveAccount: false,
			additionalFields:     []string{"businessPhones"},
			want: []interface{}{
				map[string]interface{}{
					"id":                   testUserID1,
					fieldDisplayName:       testUserDisplay,
					fieldUserPrincipalName: testUser1Email,
					fieldMail:              testUser1Email,
					fieldAccountEnabled:    true,
					"businessPhones":       []interface{}{"+1-555-0100"},
				},
			},
		},
		"AdditionalFieldsNonScalarTime": {
			reason: "A *time.Time field (createdDateTime) is normalised to RFC3339 string so it can be serialised",
			graphUsers: func() []models.Userable {
				u := models.NewUser()
				u.SetId(ptr.To(testUserID1))
				u.SetUserPrincipalName(ptr.To(testUser1Email))
				u.SetMail(ptr.To(testUser1Email))
				u.SetDisplayName(ptr.To(testUserDisplay))
				u.SetAccountEnabled(ptr.To(true))
				u.SetCreatedDateTime(ptr.To(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)))
				return []models.Userable{u}
			}(),
			requireActiveAccount: false,
			additionalFields:     []string{"createdDateTime"},
			want: []interface{}{
				map[string]interface{}{
					"id":                   testUserID1,
					fieldDisplayName:       testUserDisplay,
					fieldUserPrincipalName: testUser1Email,
					fieldMail:              testUser1Email,
					fieldAccountEnabled:    true,
					"createdDateTime":      "2024-01-02T03:04:05Z",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := &GraphQuery{log: logging.NewNopLogger()}
			got := g.buildUserResults(tc.graphUsers, tc.requireActiveAccount, tc.additionalFields)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nbuildUserResults(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestApplyAdditionalFieldsSerialisation verifies that non-scalar Graph values
// ([]string, *time.Time) survive the full serialisation path through
// putQueryResultToContext without error. This is the regression test for the
// proto: invalid type bug.
func TestApplyAdditionalFieldsSerialisation(t *testing.T) {
	u := models.NewUser()
	u.SetId(ptr.To("uid-1"))
	u.SetDisplayName(ptr.To("Test User"))
	u.SetUserPrincipalName(ptr.To("test@example.com"))
	u.SetMail(ptr.To("test@example.com"))
	u.SetAccountEnabled(ptr.To(true))
	u.SetBusinessPhones([]string{"+1-555-0100"})
	u.SetCreatedDateTime(ptr.To(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)))

	g := &GraphQuery{log: logging.NewNopLogger()}
	results := g.buildUserResults([]models.Userable{u}, false, []string{"businessPhones", "createdDateTime"})

	req := &fnv1.RunFunctionRequest{}
	rsp := &fnv1.RunFunctionResponse{}
	f := &Function{log: logging.NewNopLogger()}
	in := &v1beta1.Input{Target: "context.validatedUsers"}

	if err := putQueryResultToContext(req, rsp, in, results, f); err != nil {
		t.Fatalf("putQueryResultToContext returned unexpected error: %v", err)
	}

	got := rsp.GetContext().GetFields()["validatedUsers"].AsInterface()
	entries, ok := got.([]interface{})
	if !ok || len(entries) != 1 {
		t.Fatalf("expected 1 entry in context, got %v", got)
	}
	entry, ok := entries[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected map entry, got %T", entries[0])
	}
	if diff := cmp.Diff([]interface{}{"+1-555-0100"}, entry["businessPhones"]); diff != "" {
		t.Errorf("businessPhones: -want, +got:\n%s", diff)
	}
	if diff := cmp.Diff("2024-01-02T03:04:05Z", entry["createdDateTime"]); diff != "" {
		t.Errorf("createdDateTime: -want, +got:\n%s", diff)
	}
}

// TestSanitizeAdditionalFields verifies that sanitizeAdditionalFields drops
// entries that are not valid Graph property names and passes through valid ones.
func TestSanitizeAdditionalFields(t *testing.T) {
	g := &GraphQuery{log: logging.NewNopLogger()}

	cases := map[string]struct {
		input []string
		want  []string
	}{
		"ValidFieldsPassThrough": {
			input: []string{testFieldCity, testFieldDepartment, "extension_abc_costCenter"},
			want:  []string{testFieldCity, testFieldDepartment, "extension_abc_costCenter"},
		},
		"EmptyStringDropped": {
			input: []string{testFieldCity, "", testFieldDepartment},
			want:  []string{testFieldCity, testFieldDepartment},
		},
		"ODataInjectionDropped": {
			input: []string{"id),manager($select=id,displayName"},
			want:  []string{},
		},
		"SpaceInNameDropped": {
			input: []string{"display Name"},
			want:  []string{},
		},
		"NilInputReturnsEmpty": {
			input: nil,
			want:  []string{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := g.sanitizeAdditionalFields(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("sanitizeAdditionalFields: -want, +got:\n%s", diff)
			}
		})
	}
}

// TestApplyAdditionalFieldsByModel verifies that applyAdditionalFields correctly
// extracts typed getter values from each Graph model type used across the four
// query types. This ensures reflection-based extraction works for Group and
// ServicePrincipal in addition to User (covered by TestBuildUserResults).
func TestApplyAdditionalFieldsByModel(t *testing.T) {
	g := &GraphQuery{log: logging.NewNopLogger()}

	t.Run("GroupMailNickname", func(t *testing.T) {
		grp := models.NewGroup()
		grp.SetId(ptr.To("group-id-1"))
		grp.SetMailNickname(ptr.To("eng-team"))

		m := map[string]interface{}{}
		g.applyAdditionalFields(m, grp, "group-id-1", []string{"mailNickname"})

		if diff := cmp.Diff(map[string]interface{}{"mailNickname": "eng-team"}, m); diff != "" {
			t.Errorf("Group mailNickname: -want, +got:\n%s", diff)
		}
	})

	t.Run("GroupSecurityEnabled", func(t *testing.T) {
		grp := models.NewGroup()
		grp.SetId(ptr.To("group-id-2"))
		grp.SetSecurityEnabled(ptr.To(true))

		m := map[string]interface{}{}
		g.applyAdditionalFields(m, grp, "group-id-2", []string{"securityEnabled"})

		if diff := cmp.Diff(map[string]interface{}{"securityEnabled": true}, m); diff != "" {
			t.Errorf("Group securityEnabled: -want, +got:\n%s", diff)
		}
	})

	t.Run("ServicePrincipalType", func(t *testing.T) {
		sp := models.NewServicePrincipal()
		sp.SetId(ptr.To("sp-id-1"))
		sp.SetServicePrincipalType(ptr.To("Application"))

		m := map[string]interface{}{}
		g.applyAdditionalFields(m, sp, "sp-id-1", []string{"servicePrincipalType"})

		if diff := cmp.Diff(map[string]interface{}{"servicePrincipalType": "Application"}, m); diff != "" {
			t.Errorf("ServicePrincipal servicePrincipalType: -want, +got:\n%s", diff)
		}
	})

	t.Run("ServicePrincipalHomepage", func(t *testing.T) {
		sp := models.NewServicePrincipal()
		sp.SetId(ptr.To("sp-id-2"))
		sp.SetHomepage(ptr.To("https://example.com"))

		m := map[string]interface{}{}
		g.applyAdditionalFields(m, sp, "sp-id-2", []string{"homepage"})

		if diff := cmp.Diff(map[string]interface{}{"homepage": "https://example.com"}, m); diff != "" {
			t.Errorf("ServicePrincipal homepage: -want, +got:\n%s", diff)
		}
	})

	t.Run("GroupMemberUserDepartment", func(t *testing.T) {
		u := models.NewUser()
		u.SetId(ptr.To(testUserID1))
		u.SetDepartment(ptr.To("Engineering"))

		m := map[string]interface{}{}
		g.applyAdditionalFields(m, u, testUserID1, []string{testFieldDepartment})

		if diff := cmp.Diff(map[string]interface{}{testFieldDepartment: "Engineering"}, m); diff != "" {
			t.Errorf("GroupMembership user department: -want, +got:\n%s", diff)
		}
	})

	t.Run("UnknownFieldSkipped", func(t *testing.T) {
		grp := models.NewGroup()
		grp.SetId(ptr.To("group-id-3"))

		m := map[string]interface{}{}
		g.applyAdditionalFields(m, grp, "group-id-3", []string{"nonExistentField"})

		if len(m) != 0 {
			t.Errorf("expected empty map for unknown field, got %v", m)
		}
	})
}

// TestProcessMember exercises the real member-extraction path (bypassed by the
// mocked graphQuery in the RunFunction tests). It is the regression test for
// issue #115: typed user members must expose mail and userPrincipalName.
func TestProcessMember(t *testing.T) {
	cases := map[string]struct {
		reason string
		member models.DirectoryObjectable
		want   map[string]interface{}
	}{
		"TypedUserIncludesMailAndUPN": {
			reason: "A typed user member should expose mail and userPrincipalName from the typed getters",
			member: newTestUser(),
			want: map[string]interface{}{
				"id":                   testUserID1,
				fieldDisplayName:       "Test User 1",
				fieldType:              userType,
				fieldMail:              testUser1Email,
				fieldUserPrincipalName: testUser1Email,
			},
		},
		"TypedUserWithoutMailOmitsMail": {
			reason: "A typed user without a mail attribute should omit mail but still expose userPrincipalName",
			member: newTestUserWithoutMail(),
			want: map[string]interface{}{
				"id":                   "user-id-3",
				fieldDisplayName:       "No Mail User",
				fieldType:              userType,
				fieldUserPrincipalName: "nomail@example.com",
			},
		},
		"TypedServicePrincipalIncludesAppID": {
			reason: "A typed service principal member should expose appId from the typed getter",
			member: newTestServicePrincipal(),
			want: map[string]interface{}{
				"id":             testSPID1,
				fieldDisplayName: "Test Service Principal",
				fieldType:        servicePrincipalType,
				fieldAppID:       "sp-app-id-1",
			},
		},
		"PlainDirectoryObjectUsesAdditionalDataFallback": {
			reason: "A plain directory object should fall back to additionalData for user properties",
			member: newTestDirectoryObject(),
			want: map[string]interface{}{
				"id":                   "user-id-2",
				fieldDisplayName:       "Fallback User",
				fieldType:              userType,
				fieldMail:              testUser2Email,
				fieldUserPrincipalName: testUser2Email,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := &GraphQuery{log: logging.NewNopLogger()}
			got := g.processMember(tc.member)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nprocessMember(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// TestNormalizeGraphValue verifies that normalizeGraphValue converts non-scalar
// Graph SDK values to JSON-safe types accepted by both structpb and encoding/json.
func TestNormalizeGraphValue(t *testing.T) {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	strVal := "CC-42"
	f64Val := float64(7)

	cases := map[string]struct {
		input  interface{}
		want   interface{}
		wantOK bool
	}{
		"NilReturnsFalse": {
			input: nil, want: nil, wantOK: false,
		},
		"StringPassthrough": {
			input: "hello", want: "hello", wantOK: true,
		},
		"BoolPassthrough": {
			input: true, want: true, wantOK: true,
		},
		"Float64Passthrough": {
			input: float64(3.14), want: float64(3.14), wantOK: true,
		},
		"TimeFormattedAsRFC3339": {
			input: ts, want: "2024-01-02T03:04:05Z", wantOK: true,
		},
		"PtrTimeDeref": {
			input: &ts, want: "2024-01-02T03:04:05Z", wantOK: true,
		},
		"PtrStringDeref": {
			input: &strVal, want: "CC-42", wantOK: true,
		},
		"PtrFloat64Deref": {
			input: &f64Val, want: float64(7), wantOK: true,
		},
		"NilPtrReturnsFalse": {
			input: (*string)(nil), want: nil, wantOK: false,
		},
		"ByteSliceBase64Encoded": {
			input: []byte("hello"), want: "aGVsbG8=", wantOK: true,
		},
		"StringSliceToInterfaceSlice": {
			input: []string{"a", "b"}, want: []interface{}{"a", "b"}, wantOK: true,
		},
		"MapStringInterface": {
			input:  map[string]interface{}{"k": "v"},
			want:   map[string]interface{}{"k": "v"},
			wantOK: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := normalizeGraphValue(tc.input)
			if ok != tc.wantOK {
				t.Errorf("normalizeGraphValue(%T): wantOK=%v got=%v", tc.input, tc.wantOK, ok)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("normalizeGraphValue(%T): -want, +got:\n%s", tc.input, diff)
			}
		})
	}
}
