# function-approve

A Crossplane Composition Function for implementing manual approval workflows.

## Overview

The `function-approve` provides a serverless approval mechanism at the Crossplane level that:

1. Tracks changes to a specified field by computing a hash
2. Pauses reconciliation when changes are detected 
3. Requires explicit approval before allowing reconciliation to continue
4. Prevents drift by storing previously approved state

This function implements the approval workflow using entirely Crossplane-native mechanisms without external dependencies, making it lightweight and reliable.

## Usage

Add the function to your Crossplane installation:

```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: function-approve
spec:
  package: xpkg.upbound.io/upbound/function-approve:v0.1.0
```

### How It Works

1. When a resource is created or updated, `function-approve` calculates a hash of the monitored field (e.g., `spec.resources`).
2. The function stores this hash in `status.newHash` (or specified field).
3. If there's a previous approved hash (`status.oldHash`) and it doesn't match the new hash, reconciliation is paused.
4. An operator must approve the change by setting `status.approved = true`.
5. After approval, the new hash is stored as the approved hash, the approval flag is reset, and reconciliation resumes.
6. If a customer modifies an existing claim after approval, this will generate a new hash, requiring another approval.

## Example

```yaml
apiVersion: example.crossplane.io/v1
kind: Composition
metadata:
  name: approval-workflow-example
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1
    kind: XR
  pipeline:
  - step: require-approval
    functionRef:
      name: function-approve
    input:
      apiVersion: approve.fn.crossplane.io/v1alpha1
      kind: Input
      dataField: "spec.resources"  # Field to monitor for changes
      approvalField: "status.approved"
      newHashField: "status.newHash"
      oldHashField: "status.oldHash"
      pauseAnnotation: "crossplane.io/paused"
      detailedCondition: true
```

## Input Configuration Options

| Field | Type | Description |
|-------|------|-------------|
| `dataField` | string | **Required**. Field to monitor for changes (e.g., `spec.resources`) |
| `hashAlgorithm` | string | Algorithm to use for hash calculation. Supported values: `md5`, `sha256`, `sha512`. Default: `sha256` |
| `approvalField` | string | Status field to check for approval. Default: `status.approved` |
| `oldHashField` | string | Status field to store previously approved hash. Default: `status.oldHash` |
| `newHashField` | string | Status field to store current hash. Default: `status.newHash` |
| `pauseAnnotation` | string | Annotation to use for pausing reconciliation. Default: `crossplane.io/paused` |
| `detailedCondition` | bool | Whether to add detailed information to conditions. Default: `true` |
| `approvalMessage` | string | Message to display when approval is required. Default: `Changes detected. Approval required.` |

## Using with Custom Resources

Your XR definition must include the status fields used by the function:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xapprovals.example.crossplane.io
spec:
  group: example.crossplane.io
  names:
    kind: XApproval
    plural: xapprovals
  versions:
  - name: v1
    served: true
    referenceable: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              resources:
                type: object
                x-kubernetes-preserve-unknown-fields: true
          status:
            type: object
            properties:
              approved:
                type: boolean
                description: "Whether the current changes are approved"
              oldHash:
                type: string
                description: "Hash of previously approved resource state"
              newHash:
                type: string
                description: "Hash of current resource state"
```

## Approving Changes

When changes are detected, the resource's reconciliation is paused, and its condition will show `ApprovalRequired` status. To approve the changes, patch the resource's status:

```yaml
kubectl patch xapproval example --type=merge --subresource=status -p '{"status":{"approved":true}}'
```

After approval, the function will:
1. Record the new state as the approved state
2. Remove the pause annotation
3. Allow reconciliation to continue

## Resetting Approval State

If you need to reset the approval state, you can clear the `oldHash` field:

```yaml
kubectl patch xapproval example --type=merge --subresource=status -p '{"status":{"oldHash":""}}'
```

## Security Considerations

- Use RBAC to control who can approve changes by restricting access to the status subresource
- Consider implementing additional verification steps or multi-party approval in your workflow

## Complete Example

Here's a complete example of a composition using `function-approve`:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: approval-required-cluster
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1
    kind: XCluster
  pipeline:
  - step: require-approval
    functionRef:
      name: function-approve
    input:
      apiVersion: approve.fn.crossplane.io/v1alpha1
      kind: Input
      dataField: "spec.resources"
      approvalField: "status.approved"
      hashAlgorithm: "sha256"
      newHashField: "status.currentHash"
      oldHashField: "status.approvedHash"
      detailedCondition: true
      approvalMessage: "Cluster changes require admin approval"
  - step: create-resources
    functionRef:
      name: function-patch-and-transform
    input:
      apiVersion: pt.fn.crossplane.io/v1alpha1
      kind: Resources
      resources:
      - name: cluster
        base:
          apiVersion: eks.aws.upbound.io/v1beta1
          kind: Cluster
          spec:
            forProvider:
              region: us-west-2
              version: "1.25"
```

## Metrics and Monitoring

- Monitor resources with the `ApprovalRequired` condition to track pending approvals
- Implement alerting based on condition status for timely approvals
- Consider tracking approval times and frequencies to optimize your workflows