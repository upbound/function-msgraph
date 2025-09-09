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
	"k8s.io/utils/ptr"

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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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

// TestResolveGroupRef tests the functionality of resolving groupRef from context, status, or spec
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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

// TestResolveUsersRef tests the functionality of resolving usersRef from context, status, or spec
func TestResolveUsersRef(t *testing.T) {
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
		"UsersRefFromStatus": {
			reason: "The Function should resolve usersRef from XR status",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
		"UsersRefNotFound": {
			reason: "The Function should handle an error when usersRef cannot be resolved",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						if len(in.Users) == 0 {
							return nil, errors.New("no users provided for validation")
						}

						var results []interface{}
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
							case "user1@example.com":
								userID = "user-id-1"
								displayName = "User 1"
							case "user2@example.com":
								userID = "user-id-2"
								displayName = "User 2"
							case "admin@example.onmicrosoft.com":
								userID = "admin-id"
								displayName = "Admin User"
							default:
								userID = "test-user-id"
								displayName = "Test User"
							}

							userMap := map[string]interface{}{
								"id":                userID,
								"displayName":       displayName,
								"userPrincipalName": *user,
								"mail":              *user,
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
		"ServicePrincipalsRefFromStatus": {
			reason: "The Function should resolve servicePrincipalsRef from XR status",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
		"ServicePrincipalsRefNotFound": {
			reason: "The Function should handle an error when servicePrincipalsRef cannot be resolved",
			args: args{
				ctx: context.Background(),
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						if len(in.ServicePrincipals) == 0 {
							return nil, errors.New("no service principal names provided")
						}

						var results []interface{}
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
								spID = "sp-id-1"
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
								"id":          spID,
								"appId":       appID,
								"displayName": *sp,
								"description": description,
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
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(xr),
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
							Message: ptr.To("Target already has data, skipped query to avoid throttling"),
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "context.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "context.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"ops.crossplane.io/watched-resource": {
							Items: nil,
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "context.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"ops.crossplane.io/watched-resource": {
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
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "context.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"ops.crossplane.io/watched-resource": {
							Items: []*fnv1.Resource{
								{},
							},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"ops.crossplane.io/watched-resource": {
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
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"ops.crossplane.io/watched-resource": {
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"ops.crossplane.io/watched-resource": {
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "msgraph.fn.crossplane.io/v1alpha1",
						"kind": "Input",
						"queryType": "UserValidation",
						"users": ["user@example.com"],
						"target": "status.validatedUsers"
					}`),
					Credentials: map[string]*fnv1.Credentials{
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: creds},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"ops.crossplane.io/watched-resource": {
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
				"credentials": []byte(`{
"clientId": "test-client-id",
"clientSecret": "test-client-secret",
"subscriptionId": "test-subscription-id",
"tenantId": "test-tenant-id"
}`),
			},
		}
		workloadIdentityCredentials = &fnv1.CredentialData{
			Data: map[string][]byte{
				"credentials": []byte(`{
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
							Resource: resource.MustStructJSON(xr),
						},
					},
					Credentials: map[string]*fnv1.Credentials{
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: servicePrincipalCreds},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: servicePrincipalCreds},
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
					Meta: &fnv1.RequestMeta{Tag: "hello"},
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
						"azure-creds": {
							Source: &fnv1.Credentials_CredentialData{CredentialData: workloadIdentityCredentials},
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
