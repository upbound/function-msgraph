# function-msgraph

A Crossplane composition function for querying the Microsoft Graph API.

## Overview

The `function-msgraph` provides read-only access to Microsoft Graph API endpoints, allowing Crossplane compositions to:

1. Validate Azure AD User Existence
2. Get Group Membership
3. Get Group Object IDs
4. Get Service Principal Details

The function supports throttling mitigation with the `skipQueryWhenTargetHasData` flag to avoid unnecessary API calls.

## Usage

Add the function to your Crossplane installation:

```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: function-msgraph
spec:
  package: xpkg.upbound.io/upbound/function-msgraph:v0.1.0
```

### Azure Credentials

Create an Azure service principal with appropriate permissions to access Microsoft Graph API:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: azure-account-creds
  namespace: crossplane-system
type: Opaque
stringData:
  credentials: |
    {
      "clientId": "your-client-id",
      "clientSecret": "your-client-secret", 
      "subscriptionId": "your-subscription-id",
      "tenantId": "your-tenant-id"
    }
```

The service principal needs the following Microsoft Graph API permissions:
- User.Read.All (for user validation)
- Group.Read.All (for group operations)
- Application.Read.All (for service principal details)

## Examples

### Validate Azure AD Users

```yaml
apiVersion: example.crossplane.io/v1
kind: Composition
metadata:
  name: user-validation-example
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1
    kind: XR
  pipeline:
  - step: validate-user
    functionRef:
      name: function-msgraph
    input:
      apiVersion: msgraph.fn.crossplane.io/v1alpha1
      kind: Input
      queryType: UserValidation
      users:
        - "user1@yourdomain.com"
        - "user2@yourdomain.com"
      target: "status.validatedUsers"
      skipQueryWhenTargetHasData: true
    credentials:
      - name: azure-creds
        source: Secret
        secretRef:
          namespace: crossplane-system
          name: azure-account-creds
```

### Get Group Membership

```yaml
apiVersion: example.crossplane.io/v1
kind: Composition
metadata:
  name: group-membership-example
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1
    kind: XR
  pipeline:
  - step: get-group-members
    functionRef:
      name: function-msgraph
    input:
      apiVersion: msgraph.fn.crossplane.io/v1alpha1
      kind: Input
      queryType: GroupMembership
      group: "Developers"
      # The function will automatically select standard fields:
      # - id, displayName, mail, userPrincipalName, appId, description
      target: "status.groupMembers"
      skipQueryWhenTargetHasData: true
    credentials:
      - name: azure-creds
        source: Secret
        secretRef:
          namespace: crossplane-system
          name: azure-account-creds
```

### Get Group Object IDs

```yaml
apiVersion: example.crossplane.io/v1
kind: Composition
metadata:
  name: group-objectids-example
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1
    kind: XR
  pipeline:
  - step: get-group-objectids
    functionRef:
      name: function-msgraph
    input:
      apiVersion: msgraph.fn.crossplane.io/v1alpha1
      kind: Input
      queryType: GroupObjectIDs
      groups:
        - "Developers"
        - "Operations"
        - "Security"
      target: "status.groupObjectIDs"
      skipQueryWhenTargetHasData: true
    credentials:
      - name: azure-creds
        source: Secret
        secretRef:
          namespace: crossplane-system
          name: azure-account-creds
```

### Get Service Principal Details

```yaml
apiVersion: example.crossplane.io/v1
kind: Composition
metadata:
  name: service-principal-example
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1
    kind: XR
  pipeline:
  - step: get-service-principal-details
    functionRef:
      name: function-msgraph
    input:
      apiVersion: msgraph.fn.crossplane.io/v1alpha1
      kind: Input
      queryType: ServicePrincipalDetails
      servicePrincipals:
        - "MyServiceApp"
        - "ApiConnector"
      target: "status.servicePrincipalDetails"
      skipQueryWhenTargetHasData: true
    credentials:
      - name: azure-creds
        source: Secret
        secretRef:
          namespace: crossplane-system
          name: azure-account-creds
```

## Input Configuration Options

| Field | Type | Description |
|-------|------|-------------|
| `queryType` | string | Required. Type of query to perform. Valid values: `UserValidation`, `GroupMembership`, `GroupObjectIDs`, `ServicePrincipalDetails` |
| `users` | []string | List of user principal names (email IDs) for user validation |
| `usersRef` | string | Reference to resolve a list of user names from `spec`, `status` or `context` (e.g., `spec.userAccess.emails`) |
| `group` | string | Single group name for group membership queries |
| `groupRef` | string | Reference to resolve a single group name from `spec`, `status` or `context` (e.g., `spec.groupConfig.name`) |
| `groups` | []string | List of group names for group object ID queries |
| `groupsRef` | string | Reference to resolve a list of group names from `spec`, `status` or `context` (e.g., `spec.groupConfig.names`) |
| `servicePrincipals` | []string | List of service principal names |
| `servicePrincipalsRef` | string | Reference to resolve a list of service principal names from `spec`, `status` or `context` (e.g., `spec.servicePrincipalConfig.names`) |
| `target` | string | Required. Where to store the query results. Can be `status.<field>` or `context.<field>` |
| `skipQueryWhenTargetHasData` | bool | Optional. When true, will skip the query if the target already has data |

## Result Targets

Results can be stored in either XR Status or Composition Context:

```yaml
# Store in XR Status
target: "status.results"

# Store in nested XR Status
target: "status.nested.field.results"

# Store in Composition Context
target: "context.results"

# Store in Environment
target: "context.[apiextensions.crossplane.io/environment].results"
```

## Using Reference Fields

You can reference values from XR spec, status, or context instead of hardcoding them:

### Using groupRef from spec

```yaml
apiVersion: msgraph.fn.crossplane.io/v1alpha1
kind: Input
queryType: GroupMembership
groupRef: "spec.groupConfig.name"  # Get group name from XR spec
target: "status.groupMembers"
```

### Using groupsRef from spec

```yaml
apiVersion: msgraph.fn.crossplane.io/v1alpha1
kind: Input
queryType: GroupObjectIDs
groupsRef: "spec.groupConfig.names"  # Get group names from XR spec
target: "status.groupObjectIDs"
```

### Using usersRef from spec

```yaml
apiVersion: msgraph.fn.crossplane.io/v1alpha1
kind: Input
queryType: UserValidation
usersRef: "spec.userAccess.emails"  # Get user emails from XR spec
target: "status.validatedUsers"
```

### Using servicePrincipalsRef from spec

```yaml
apiVersion: msgraph.fn.crossplane.io/v1alpha1
kind: Input
queryType: ServicePrincipalDetails
servicePrincipalsRef: "spec.servicePrincipalConfig.names"  # Get service principal names from XR spec
target: "status.servicePrincipals"
```

## References

- [Microsoft Graph API Overview](https://learn.microsoft.com/en-us/graph/api/overview?view=graph-rest-1.0)
- [User validation](https://learn.microsoft.com/en-us/graph/api/user-list?view=graph-rest-1.0&tabs=go)
- [Group membership](https://learn.microsoft.com/en-us/graph/api/group-list-members?view=graph-rest-1.0&tabs=go)
- [Group listing](https://learn.microsoft.com/en-us/graph/api/group-list?view=graph-rest-1.0&tabs=go)
- [Service principal listing](https://learn.microsoft.com/en-us/graph/api/serviceprincipal-list?view=graph-rest-1.0&tabs=http)
