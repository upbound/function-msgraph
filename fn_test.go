package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/upbound/function-msgraph/input/v1beta1"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

type MockGraphQuery struct {
	GraphQueryFunc func(ctx context.Context, azureCreds map[string]string, in *v1beta1.Input) (interface{}, error)
}

func (m *MockGraphQuery) graphQuery(ctx context.Context, azureCreds map[string]string, in *v1beta1.Input) (interface{}, error) {
	return m.GraphQueryFunc(ctx, azureCreds, in)
}

func strPtr(s string) *string {
	return &s
}

// TestResolveGroupsRef tests the functionality of resolving groupsRef from context or status
func TestResolveGroupsRef(t *testing.T) {
	var (
		xr    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
		creds = &fnv1.CredentialData{
			Data: map[string][]byte{
				"credentials": []byte(`{
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  `QueryType: "GroupObjectIDs"`,
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  `QueryType: "GroupObjectIDs"`,
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
		"GroupsRefNotFound": {
			reason: "The Function should handle an error when groupsRef cannot be resolved",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
						if len(in.Groups) == 0 {
							return nil, errors.New("no group names provided")
						}

						var results []interface{}
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
								"id":          groupID,
								"displayName": *group,
								"description": description,
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

// TestResolveGroupRef tests the functionality of resolving groupRef from context or status
func TestResolveGroupRef(t *testing.T) {
	var (
		xr    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
		creds = &fnv1.CredentialData{
			Data: map[string][]byte{
				"credentials": []byte(`{
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
								"id":                "user-id-1",
								"displayName":       "Test User 1",
								"mail":              "user1@example.com",
								"userPrincipalName": "user1@example.com",
								"type":              "user",
							},
							map[string]interface{}{
								"id":          "sp-id-1",
								"displayName": "Test Service Principal",
								"appId":       "sp-app-id-1",
								"type":        "servicePrincipal",
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

func TestRunFunction(t *testing.T) {

	var (
		xr    = `{"apiVersion":"example.org/v1","kind":"XR","metadata":{"name":"cool-xr"},"spec":{"count":2}}`
		creds = &fnv1.CredentialData{
			Data: map[string][]byte{
				"credentials": []byte(`{
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"]
					}`),
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
								"apiVersion": "",
								"kind": ""
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"]
					}`),
					Credentials: map[string]*fnv1.Credentials{
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
								"apiVersion": "",
								"kind": ""
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  `QueryType: "UserValidation"`,
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
		"GroupMembershipMissingGroup": {
			reason: "The Function should handle GroupMembership with missing group",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  `QueryType: "GroupObjectIDs"`,
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  `QueryType: "ServicePrincipalDetails"`,
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:    "FunctionSkip",
							Message: strPtr("Target already has data, skipped query to avoid throttling"),
							Status:  fnv1.Status_STATUS_CONDITION_TRUE,
							Reason:  "SkippedQuery",
							Target:  fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
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
		"QueryToContextField": {
			reason: "The Function should store results in context field",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Results: []*fnv1.Result{
						{
							Severity: fnv1.Severity_SEVERITY_NORMAL,
							Message:  `QueryType: "UserValidation"`,
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
					switch in.QueryType {
					case "UserValidation":
						if len(in.Users) == 0 {
							return nil, errors.New("no users provided for validation")
						}
						return []interface{}{
							map[string]interface{}{
								"id":                "test-user-id",
								"displayName":       "Test User",
								"userPrincipalName": "user@example.com",
								"mail":              "user@example.com",
							},
						}, nil
					case "GroupMembership":
						if in.Group == nil || *in.Group == "" {
							return nil, errors.New("no group name provided")
						}
						return []interface{}{
							map[string]interface{}{
								"id":                "user-id-1",
								"displayName":       "Test User 1",
								"mail":              "user1@example.com",
								"userPrincipalName": "user1@example.com",
								"type":              "user",
							},
							map[string]interface{}{
								"id":          "sp-id-1",
								"displayName": "Test Service Principal",
								"appId":       "sp-app-id-1",
								"type":        "servicePrincipal",
							},
						}, nil
					case "GroupObjectIDs":
						if len(in.Groups) == 0 {
							return nil, errors.New("no group names provided")
						}
						return []interface{}{
							map[string]interface{}{
								"id":          "group-id-1",
								"displayName": "Developers",
								"description": "Development team",
							},
							map[string]interface{}{
								"id":          "group-id-2",
								"displayName": "Operations",
								"description": "Operations team",
							},
						}, nil
					case "ServicePrincipalDetails":
						if len(in.ServicePrincipals) == 0 {
							return nil, errors.New("no service principal names provided")
						}
						return []interface{}{
							map[string]interface{}{
								"id":          "sp-id-1",
								"appId":       "app-id-1",
								"displayName": "MyServiceApp",
								"description": "Service application",
							},
						}, nil
					default:
						return nil, errors.Errorf("unsupported query type: %s", in.QueryType)
					}
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
