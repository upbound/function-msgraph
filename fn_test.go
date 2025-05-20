package main

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
)

func TestFunction_MalformedInput(t *testing.T) {
	f := &Function{
		log: logging.NewNopLogger(),
	}

	req := &fnv1.RunFunctionRequest{
		Meta: &fnv1.RequestMeta{Tag: "fn-approval"},
		Input: resource.MustStructJSON(`{
			"apiVersion": "approve.fn.crossplane.io/v1alpha1",
			"kind": "Input"
		}`),
		Observed: &fnv1.State{
			Composite: &fnv1.Resource{
				Resource: resource.MustStructJSON(`{
					"apiVersion": "example.org/v1",
					"kind": "XR",
					"metadata": {
						"name": "test-xr"
					}
				}`),
			},
		},
	}

	rsp, err := f.RunFunction(context.Background(), req)

	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}

	if rsp == nil {
		t.Fatal("expected response but got nil")
	}

	// Should have a fatal error result
	if len(rsp.Results) == 0 {
		t.Fatal("expected at least one result but got none")
	}

	// The first result should be a fatal error
	if rsp.Results[0].Severity != fnv1.Severity_SEVERITY_FATAL {
		t.Errorf("expected SEVERITY_FATAL but got: %v", rsp.Results[0].Severity)
	}
}

func TestFunction_FirstRunNeedsApproval(t *testing.T) {
	f := &Function{
		log: logging.NewNopLogger(),
	}

	req := &fnv1.RunFunctionRequest{
		Meta: &fnv1.RequestMeta{Tag: "fn-approval"},
		Input: resource.MustStructJSON(`{
			"apiVersion": "approve.fn.crossplane.io/v1alpha1",
			"kind": "Input",
			"dataField": "spec.resources",
			"approvalField": "status.approved",
			"oldHashField": "status.oldHash",
			"newHashField": "status.newHash",
			"detailedCondition": true
		}`),
		Observed: &fnv1.State{
			Composite: &fnv1.Resource{
				Resource: resource.MustStructJSON(`{
					"apiVersion": "example.org/v1",
					"kind": "XR",
					"metadata": {
						"name": "test-xr"
					},
					"spec": {
						"resources": {
							"test": "data"
						}
					}
				}`),
			},
		},
	}

	rsp, err := f.RunFunction(context.Background(), req)

	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}

	if rsp == nil {
		t.Fatal("expected response but got nil")
	}

	// Should have no error results
	if len(rsp.Results) > 0 {
		t.Errorf("expected no results but got: %v", rsp.Results)
	}

	// Should have an approval required condition
	if len(rsp.Conditions) == 0 {
		t.Fatal("expected at least one condition but got none")
	}

	// The first condition should be ApprovalRequired
	if rsp.Conditions[0].Type != "ApprovalRequired" {
		t.Errorf("expected ApprovalRequired condition but got: %v", rsp.Conditions[0].Type)
	}

	if rsp.Conditions[0].Status != fnv1.Status_STATUS_CONDITION_FALSE {
		t.Errorf("expected STATUS_CONDITION_FALSE but got: %v", rsp.Conditions[0].Status)
	}

	if rsp.Conditions[0].Reason != "WaitingForApproval" {
		t.Errorf("expected WaitingForApproval reason but got: %v", rsp.Conditions[0].Reason)
	}
}

func TestFunction_AlreadyApproved(t *testing.T) {
	f := &Function{
		log: logging.NewNopLogger(),
	}

	const hashValue = "e02c6d35c585a43c62dc2ae14a5385b8a86168b36be7a0d985c0c09afca4ffbe"

	req := &fnv1.RunFunctionRequest{
		Meta: &fnv1.RequestMeta{Tag: "fn-approval"},
		Input: resource.MustStructJSON(`{
			"apiVersion": "approve.fn.crossplane.io/v1alpha1",
			"kind": "Input",
			"dataField": "spec.resources",
			"approvalField": "status.approved",
			"oldHashField": "status.oldHash",
			"newHashField": "status.newHash"
		}`),
		Observed: &fnv1.State{
			Composite: &fnv1.Resource{
				Resource: resource.MustStructJSON(`{
					"apiVersion": "example.org/v1",
					"kind": "XR",
					"metadata": {
						"name": "test-xr",
						"annotations": {
							"crossplane.io/paused": "true"
						}
					},
					"spec": {
						"resources": {
							"test": "data"
						}
					},
					"status": {
						"approved": true,
						"newHash": "` + hashValue + `"
					}
				}`),
			},
		},
	}

	rsp, err := f.RunFunction(context.Background(), req)

	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}

	if rsp == nil {
		t.Fatal("expected response but got nil")
	}

	// Should have no error results
	if len(rsp.Results) > 0 {
		t.Errorf("expected no results but got: %v", rsp.Results)
	}

	// Should have a success condition
	if len(rsp.Conditions) == 0 {
		t.Fatal("expected at least one condition but got none")
	}

	// The first condition should be FunctionSuccess
	if rsp.Conditions[0].Type != "FunctionSuccess" {
		t.Errorf("expected FunctionSuccess condition but got: %v", rsp.Conditions[0].Type)
	}

	if rsp.Conditions[0].Status != fnv1.Status_STATUS_CONDITION_TRUE {
		t.Errorf("expected STATUS_CONDITION_TRUE but got: %v", rsp.Conditions[0].Status)
	}

	if rsp.Conditions[0].Reason != "Success" {
		t.Errorf("expected Success reason but got: %v", rsp.Conditions[0].Reason)
	}
}

func TestFunction_ChangesRequireApproval(t *testing.T) {
	f := &Function{
		log: logging.NewNopLogger(),
	}

	const oldHash = "e02c6d35c585a43c62dc2ae14a5385b8a86168b36be7a0d985c0c09afca4ffbe"

	req := &fnv1.RunFunctionRequest{
		Meta: &fnv1.RequestMeta{Tag: "fn-approval"},
		Input: resource.MustStructJSON(`{
			"apiVersion": "approve.fn.crossplane.io/v1alpha1",
			"kind": "Input",
			"dataField": "spec.resources",
			"approvalField": "status.approved",
			"oldHashField": "status.oldHash",
			"newHashField": "status.newHash"
		}`),
		Observed: &fnv1.State{
			Composite: &fnv1.Resource{
				Resource: resource.MustStructJSON(`{
					"apiVersion": "example.org/v1",
					"kind": "XR",
					"metadata": {
						"name": "test-xr"
					},
					"spec": {
						"resources": {
							"test": "updated-data"
						}
					},
					"status": {
						"approved": false,
						"oldHash": "` + oldHash + `"
					}
				}`),
			},
		},
	}

	rsp, err := f.RunFunction(context.Background(), req)

	if err != nil {
		t.Errorf("expected no error but got: %v", err)
	}

	if rsp == nil {
		t.Fatal("expected response but got nil")
	}

	// Should have no error results
	if len(rsp.Results) > 0 {
		t.Errorf("expected no results but got: %v", rsp.Results)
	}

	// Should have an approval required condition
	if len(rsp.Conditions) == 0 {
		t.Fatal("expected at least one condition but got none")
	}

	// The first condition should be ApprovalRequired
	if rsp.Conditions[0].Type != "ApprovalRequired" {
		t.Errorf("expected ApprovalRequired condition but got: %v", rsp.Conditions[0].Type)
	}

	if rsp.Conditions[0].Status != fnv1.Status_STATUS_CONDITION_FALSE {
		t.Errorf("expected STATUS_CONDITION_FALSE but got: %v", rsp.Conditions[0].Status)
	}

	if rsp.Conditions[0].Reason != "WaitingForApproval" {
		t.Errorf("expected WaitingForApproval reason but got: %v", rsp.Conditions[0].Reason)
	}
}
