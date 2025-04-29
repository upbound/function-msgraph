# Microsoft Graph API Function Examples

This directory contains practical examples that demonstrate the function-msgraph capabilities for querying Microsoft Graph API.

## Prerequisites

To run these examples, you need:

1. The Crossplane CLI installed
2. Valid Azure credentials with Microsoft Graph API permissions:
   - User.Read.All (for user validation)
   - Group.Read.All (for group operations)
   - Application.Read.All (for service principal details)

## Update Credentials

Before running any examples, update `secrets/azure-creds.yaml` with your valid Azure credentials:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: azure-account-creds
type: Opaque
stringData:
  credentials: |
    {
      "clientId": "your-client-id",
      "clientSecret": "your-client-secret",
      "tenantId": "your-tenant-id",
      "subscriptionId": "your-subscription-id"
    }
```

## Core Examples

### 1. User Validation

Validate if specified Azure AD users exist:

```shell
crossplane render xr.yaml user-validation-example.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
```

Dynamic `usersRef` variations:

```shell
crossplane render xr.yaml user-validation-example-status-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
```

```shell
crossplane render xr.yaml user-validation-example-context-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc --extra-resources=envconfig.yaml
```

### 2. Group Membership

Get all members of a specified Azure AD group:

```shell
crossplane render xr.yaml group-membership-example.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
```

Dynamic `groupRef` variations:

```shell
crossplane render xr.yaml group-membership-example-status-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
```

```shell
crossplane render xr.yaml group-membership-example-context-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc --extra-resources=envconfig.yaml
```

### 3. Group Object IDs

Get object IDs for specified Azure AD groups:

```shell
crossplane render xr.yaml group-objectids-example.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
```

Dynamic `groupsRef` variations:

```shell
crossplane render xr.yaml group-objectids-example-status-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
```

```shell
crossplane render xr.yaml group-objectids-example-context-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc --extra-resources=envconfig.yaml
```

### 4. Service Principal Details

Get details of specified service principals:

```shell
crossplane render xr.yaml service-principal-example.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
```

Dynamic `servicePrinicpalsRef` variations:

```shell
crossplane render xr.yaml service-principal-example-status-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
```

```shell
crossplane render xr.yaml service-principal-example-context-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc --extra-resources=envconfig.yaml
```
