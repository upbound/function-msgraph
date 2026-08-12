# function-msgraph e2e (Composition mode)

Manifests to exercise features end-to-end on a real cluster, where behaviour
across reconciles can actually be observed (something `crossplane render` cannot
show).

## Files

| File | Purpose |
|------|---------|
| `function.yaml` | Installs `function-msgraph` (pin the tag you want to test) with `--debug` enabled |
| `composition.yaml` | UserValidation → `status.validatedUsers`, `queryInterval: "2m"` |
| `xr.yaml` | A composite resource instance for the `queryInterval` scenario |
| `composition-active-account.yaml` | Two UserValidation Compositions, identical except for `activeAccount: true` |
| `xr-active-account.yaml` | One composite resource instance per `activeAccount` Composition |

Reuses `../definition.yaml` (XRD) and the `azure-account-creds` secret built from
`../secrets/azure-creds.yaml` — see [Update Credentials](../README.md#update-credentials).
Populate it locally; keep real values out of commits.

## Cluster

```shell
kind create cluster --name msgraph-e2e
helm install crossplane crossplane-stable/crossplane \
  -n crossplane-system --create-namespace --version 2.3.3 --wait

kubectl apply -f example/e2e/function.yaml
kubectl wait function.pkg.crossplane.io/function-msgraph --for=condition=Healthy --timeout=180s

kubectl apply -f example/definition.yaml
kubectl apply -f example/secrets/azure-creds.yaml   # populate locally; keep real values out of commits
```

## Scenario: queryInterval

```shell
kubectl apply -f example/e2e/composition.yaml
kubectl apply -f example/e2e/xr.yaml
```

```shell
# the result list stays a clean list of results
kubectl get xr msgraph-query-interval-e2e -o jsonpath='{.status.validatedUsers}' | jq

# the query timestamp is recorded separately, keyed by target
kubectl get xr msgraph-query-interval-e2e -o jsonpath='{.status.lastQueryTimestamps}' | jq

# within the interval, reconciles skip (condition is set on the XR)
kubectl get xr msgraph-query-interval-e2e -o jsonpath='{.status.conditions}' | jq   # FunctionSkip/IntervalLimit

# count real Graph calls vs skips in the function logs
POD=$(kubectl get pods -n crossplane-system -o name | grep msgraph)
kubectl logs -n crossplane-system "$POD" | grep -c 'Query Type'      # real queries
kubectl logs -n crossplane-system "$POD" | grep -c 'interval limit'  # skips
```

Force reconciles with `kubectl annotate xr msgraph-query-interval-e2e poke=$(date +%s) --overwrite`.
Poking repeatedly inside the 2m window leaves the query count flat; after 2m
elapses the next reconcile re-queries and `status.lastQueryTimestamps.validatedUsers` advances.

## Scenario: activeAccount

Edit `composition-active-account.yaml` and `xr-active-account.yaml` first: replace
the placeholder UPNs with one enabled and one deliberately disabled account from
your directory, otherwise the filter has nothing to exclude.

```shell
kubectl apply -f example/e2e/composition-active-account.yaml
kubectl apply -f example/e2e/xr-active-account.yaml
```

```shell
# baseline: the disabled account passes validation, reported as accountEnabled false
kubectl get xr msgraph-baseline-e2e -o jsonpath='{.status.validatedUsers}' | jq

# activeAccount: only the enabled account is stored
kubectl get xr msgraph-active-account-e2e -o jsonpath='{.status.validatedUsers}' | jq

# the exclusion is logged per user
POD=$(kubectl get pods -n crossplane-system -o name | grep msgraph)
kubectl logs -n crossplane-system "$POD" | grep 'Skipping user with disabled account'
```

Both XRs should report `SYNCED=True READY=True` with a `FunctionSuccess/Success`
condition. Poke either XR to confirm the filtering holds across reconciles rather
than being a first-pass artefact.

## Notes

- `queryInterval` is effective only in Composition mode with a `status.` target.
- `activeAccount` filters the result of a query, it does not purge a target that
  is no longer being refreshed. Combining it with `skipQueryWhenTargetHasData` or
  `queryInterval` leaves already-stored disabled users in place, which is why the
  `activeAccount` Composition sets neither.
- The example XRD resolves to `LegacyCluster` scope under Crossplane v2, so the XR
  is cluster-scoped (no namespace).
- Teardown: `kind delete cluster --name msgraph-e2e`.
