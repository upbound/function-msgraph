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

```shell
crossplane render xr.yaml user-validation-example-spec-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
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

```shell
crossplane render xr.yaml group-membership-example-spec-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
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

```shell
crossplane render xr.yaml group-objectids-example-spec-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
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

```shell
crossplane render xr.yaml service-principal-example-spec-ref.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -rc
```

### 5. Query Interval (Throttling)

Throttle calls to Microsoft Graph with `queryInterval` (a Go duration string, e.g. `10m`). On a successful query the function records a timestamp under `status.lastQueryTimestamps` (keyed by target), leaving the result list clean, and on later reconciles skips querying until the interval has elapsed. It is only effective in Composition mode with a `status.` target.

Run the query and observe the recorded timestamp under `status.lastQueryTimestamps` (the result list at `status.validatedUsers` stays a plain list):

```shell
crossplane render xr.yaml user-validation-example-query-interval.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -r
```

To observe the skip, render against an XR that already records a recent timestamp in `status.lastQueryTimestamps`. `crossplane render` treats the XR file as the observed composite, so the function reads that timestamp and, while the interval has not elapsed, skips the query and emits a `FunctionSkip`/`IntervalLimit` condition instead of calling Graph:

```shell
crossplane render xr-with-last-query-time.yaml user-validation-example-query-interval.yaml functions.yaml --function-credentials=./secrets/azure-creds.yaml -r
```

> `xr-with-last-query-time.yaml` uses a far-future timestamp in `status.lastQueryTimestamps` so the skip is deterministic; set it to a real recent time to test the natural elapsed boundary. The credentials are not used on the skip path (no Graph call is made), so they need not be valid for this command.

> **macOS note:** these commands use `-r` (function results) rather than `-rc`. The `-c`/`--include-context` flag makes `crossplane render` v2.x run an internal context-extraction step over a unix socket bind-mounted into its Docker helper container, which Docker Desktop for macOS does not support (`connect: operation not supported`) — the render then hangs with no output. Since the query-interval results are written to a `status.` target, `-c` is unnecessary here. This is a known CLI bug ([crossplane/cli#161](https://github.com/crossplane/cli/issues/161)), fixed by [#163](https://github.com/crossplane/cli/pull/163) (context function now listens on TCP) but not yet in a tagged release as of CLI v2.4.0. Until then, drop `-c` on macOS, or run `crossplane render` on Linux / a CLI built from `main` if you need context output.
