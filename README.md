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

The service principal needs the following Microsoft Graph API permissions:
- User.Read.All (for user validation)
- Group.Read.All (for group operations)
- Application.Read.All (for service principal details)

#### Client Secret Credentials
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

#### Workload Identity Credentials
AKS cluster needs to have workload identity enabled.
The managed identity needs to have the Federated Identity Credential created: https://azure.github.io/azure-workload-identity/docs/topics/federated-identity-credential.html.

##### Credentials secret:
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
      "clientId": "your-client-id", # optional
      "tenantId": "your-tenant-id", # optional
      "federatedTokenFile": "/var/run/secrets/azure/tokens/azure-identity-token"
    }
```

##### Function
```yaml
apiVersion: pkg.crossplane.io/v1
kind: Function
metadata:
  name: upbound-function-msgraph
spec:
  package: xpkg.upbound.io/upbound/function-msgraph:v0.2.0
  runtimeConfigRef:
    apiVersion: pkg.crossplane.io/v1beta1
    kind: DeploymentRuntimeConfig
    name: upbound-function-msgraph
```

##### DeploymentRuntimeConfig
```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: DeploymentRuntimeConfig
metadata:
  name: upbound-function-msgraph
spec: 
  deploymentTemplate:
    spec:
      selector:
        matchLabels:
          azure.workload.identity/use: "true"
          pkg.crossplane.io/function: "upbound-function-msgraph"
      template:
        metadata:
          labels:
            azure.workload.identity/use: "true"
            pkg.crossplane.io/function: "upbound-function-msgraph"
        spec:
          containers:
          - name: package-runtime
            volumeMounts:
            - mountPath: /var/run/secrets/azure/tokens
              name: azure-identity-token
              readOnly: true
          serviceAccountName: "upbound-function-msgraph"
          volumes:
          - name: azure-identity-token
            projected:
              sources:
              - serviceAccountToken:
                  audience: api://AzureADTokenExchange
                  expirationSeconds: 3600
                  path: azure-identity-token
  serviceAccountTemplate:
    metadata:
      annotations:
        azure.workload.identity/client-id: "your-client-id"
      name: "upbound-function-msgraph"
```

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
| `queryInterval` | string | Optional. Minimum interval between queries as a Go duration string (e.g. `10m`, `1h`, `90s`). Skips querying Microsoft Graph until the interval has elapsed since the last successful query, independent of reconcile frequency. Only effective in Composition mode with a `status.` target. |
| `FailOnEmpty` | bool | Optional. When true, the function will fail if the `users`, `groups`, or `servicePrincipals` lists are empty, or if their respective reference fields are empty lists. |
| `identity.type` | string | Optional. Type of identity credentials to use. Valid values: `AzureServicePrincipalCredentials`, `AzureWorkloadIdentityCredentials`. Default is `AzureServicePrincipalCredentials` |

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

## Throttling Mitigation

Microsoft Graph enforces API request throttling. The function offers two
complementary controls to reduce how often it calls the Graph API:

- `skipQueryWhenTargetHasData` — skips the query whenever the target already
  holds data. Useful when the data only needs to be fetched once.
- `queryInterval` — skips the query until a minimum interval has elapsed since
  the last successful query, then refreshes the data. Useful when the data
  should be kept reasonably fresh without querying on every reconcile.

These are intended as alternative strategies. If both are set, the interval is
checked first, but `skipQueryWhenTargetHasData` takes over once the target holds
data, so the interval refresh never runs. Pick one per pipeline step; the
function emits a non-fatal warning when both are configured.

### Query Interval

`queryInterval` accepts a Go duration string (for example `10m`, `1h` or `90s`).
When set, the function records the timestamp of its last successful query
alongside the result stored at the status target and skips querying Microsoft
Graph again until the interval has elapsed — regardless of how frequently the
Composition reconciles.

```yaml
target: "status.validatedUsers"
queryInterval: "10m"
```

The timestamp is persisted as an extra `lastQueryTime` element appended to the
result list at the status target, for example:

```yaml
status:
  validatedUsers:
    - id: "a1b2c3"
      displayName: "Jane Doe"
      userPrincipalName: "jane@example.com"
      mail: "jane@example.com"
    - lastQueryTime: "2026-07-16T10:00:00Z"
```

Because the interval check reads the persisted timestamp back from the XR status
on subsequent reconciles, `queryInterval` is only effective in **Composition
mode with a `status.` target**. It has no effect for `context.` targets (context
is not persisted across reconciles) or in Operation mode, where the query
cadence is controlled by the `CronOperation` schedule or `WatchOperation`
instead.

> Note: consumers of the result list should ignore the trailing element that
> carries `lastQueryTime` (it has no `id`), as it is metadata rather than a query
> result.

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

## Using data from EnvironmentConfig

To use data from a Crossplane `EnvironmentConfig`, first load the data into the context using [function-environment-configs](https://github.com/crossplane-contrib/function-environment-configs). This function should be in the pipeline before you use function-msgraph.

function-environment-configs loads a merged map of environment config data to `context["apiextensions.crossplane.io/environment"]` but to refer to this correctly in function-msgraph the following syntax must be used: `context.[apiextensions.crossplane.io/environment]`.

### Example:

1. First, define your Environment Config:

```yaml
apiVersion: apiextensions.crossplane.io/v1beta1
kind: EnvironmentConfig
metadata:
  name: example-config
data:
  entraid:
    users:
      - user@example.com
```

2. Load the Environment Config in to your composition pipeline:

```yaml
  pipeline:
    - step: load-environment-config
      functionRef:
        name: crossplane-contrib-function-environment-configs
      input:
        apiVersion: environmentconfigs.fn.crossplane.io/v1beta1
        kind: Input
        spec:
          environmentConfigs:
            - type: Reference
              ref:
                name: example-config
```

3. Reference the Environment Config data in your function input:

```yaml
    - step: load-environment-config
      # ... omitted for brevity
    - step: get-group-members
      functionRef:
        name: function-msgraph
      input:
        apiVersion: msgraph.fn.crossplane.io/v1alpha1
        kind: Input
        queryType: GroupMembership
        groupRef: context.[apiextensions.crossplane.io/environment].entraid.users
        target: "status.users"
```

## Using Different Credentials

### Using ServicePrincipal credentials

#### Explicitly
```yaml
apiVersion: msgraph.fn.crossplane.io/v1alpha1
kind: Input
identity:
  type: AzureServicePrincipalCredentials
```

#### Default
```yaml
apiVersion: msgraph.fn.crossplane.io/v1alpha1
kind: Input
```

### Using Workload Identity Credentials
```yaml
apiVersion: msgraph.fn.crossplane.io/v1alpha1
kind: Input
identity:
  type: AzureWorkloadIdentityCredentials
```

## Operations support
function-msgraph support every kind of [operations](https://docs.crossplane.io/latest/operations/operation/) but it only allows targeting Composite Resources
Function omits the input.skipQueryWhenTargetHasData parameter when running in operation mode to enforce compability with Cron/Watch modes.
CronOperations and WatchOperations are the most useful in context of graph queries, please check [examples](./example/operations/).
### Operations and Compositions Working Together

**Important**: Operations and Compositions work in conjunction to provide a self-healing mechanism:

1. **Operations Role (Drift Detection)**:
   - Query Microsoft Graph API on schedule/watch events
   - Compare results with current XR status
   - Set drift detection annotations (but don't update status directly)

2. **Compositions Role (Drift Correction)**:
   - Run when XR is reconciled (triggered by annotation changes)
   - Check drift detection annotation
   - If drift detected, ignore `skipQueryWhenTargetHasData` flag and update status
   - Reset drift annotation to "false" after successful update

This creates a **two-phase self-healing system** where Operations monitor for changes and Compositions perform the actual data updates.
### Operations results
function-msgraph operations result in two annotations set on the XR:
```yaml
apiVersion: "example.org/v1"
kind: XR
metadata:
  name: "cool-xr"
  annotations:
    "function-msgraph/last-execution": "2025-01-01T00:00:00+01:00"
    "function-msgraph/last-execution-query-drift-detected": "false"
```
function-msgraph/last-execution sets RFC3339 timestamp informing about last succesful Operation run.
function-msgraph/last-execution-query-drift-detected sets a boolean if there's a drift between input.target field's value and query result, which is used by function-msgraph in Composition context for self-healing. skipQueryWhenTargetHasData input parameter is ommited when drift detected annotation is set which leads to XR update and after that next Operation run sets the annotation back to "false".

### CronOperation
CronOperation may be used to forcefully update XR's status in a predefined interval.
That functionality may be especially useful for XRs that are business critical and should have the data refreshed without worrying about throttling.
Supports only singular resource reference.

```yaml
apiVersion: ops.crossplane.io/v1alpha1
kind: CronOperation
metadata:
  name: update-user-validation-for-critical-xr
spec:
  schedule: "*/5 * * * *" # Every 5 minutes
  concurrencyPolicy: Forbid
  successfulHistoryLimit: 5
  failedHistoryLimit: 3
  operationTemplate:
    spec:
      mode: Pipeline
      pipeline:
      - step: user-validation
        functionRef:
          name: function-msgraph
        input:
          apiVersion: msgraph.fn.crossplane.io/v1alpha1
          kind: Input
          queryType: UserValidation
          # Replace these with actual users in your directory
          users:
            - "admin@example.onmicrosoft.com"
            - "user@example.onmicrosoft.com"
            - "yury@upbound.io"
          target: "status.validatedUsers"
        credentials:
          - name: azure-creds
            source: Secret
            secretRef:
              namespace: crossplane-system
              name: azure-account-creds
        requirements:
          requiredResources:
          - requirementName: ops.crossplane.io/watched-resource
            apiVersion: example.crossplane.io/v1
            kind: XR
            name: business-critical-xr
```
### WatchOperation
WatchOperation may be used to forcefully update XR's status based on match condition.
For example it may be useful to refresh status in business critical XR's that are labeled with label `always-update: "true"`.
```yaml
apiVersion: ops.crossplane.io/v1alpha1
kind: WatchOperation
metadata:
  name: update-user-validation-for-critical-xrs
spec:
  watch:
    apiVersion: example.crossplane.io/v1
    kind: XR
    matchLabels:
      always-update: "true"
  concurrencyPolicy: Allow
  operationTemplate:
    spec:
      mode: Pipeline
      pipeline:
      - step: user-validation
        functionRef:
          name: function-msgraph
        input:
          apiVersion: msgraph.fn.crossplane.io/v1alpha1
          kind: Input
          queryType: UserValidation
          # Replace these with actual users in your directory
          users:
            - "admin@example.onmicrosoft.com"
            - "user@example.onmicrosoft.com"
            - "yury@upbound.io"
          target: "status.validatedUsers"
        credentials:
          - name: azure-creds
            source: Secret
            secretRef:
              namespace: crossplane-system
              name: azure-account-creds
```

## References

- [Microsoft Graph API Overview](https://learn.microsoft.com/en-us/graph/api/overview?view=graph-rest-1.0)
- [User validation](https://learn.microsoft.com/en-us/graph/api/user-list?view=graph-rest-1.0&tabs=go)
- [Group membership](https://learn.microsoft.com/en-us/graph/api/group-list-members?view=graph-rest-1.0&tabs=go)
- [Group listing](https://learn.microsoft.com/en-us/graph/api/group-list?view=graph-rest-1.0&tabs=go)
- [Service principal listing](https://learn.microsoft.com/en-us/graph/api/serviceprincipal-list?view=graph-rest-1.0&tabs=http)
