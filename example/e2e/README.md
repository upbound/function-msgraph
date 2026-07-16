# queryInterval e2e (Composition mode)

Manifests to exercise the `queryInterval` throttling feature end-to-end on a real
cluster, where the time-based skip/refresh loop can actually be observed across
reconciles (something `crossplane render` cannot show).

## Files

| File | Purpose |
|------|---------|
| `function.yaml` | Installs `function-msgraph` (pin the tag you want to test) |
| `composition.yaml` | UserValidation → `status.validatedUsers`, `queryInterval: "2m"` |
| `xr.yaml` | A composite resource instance |

Reuses `../definition.yaml` (XRD) and the `azure-account-creds` secret built from
`../secrets/azure-creds.yaml` — see [Update Credentials](../README.md#update-credentials).
Populate it locally; keep real values out of commits.

## Run

```shell
kind create cluster --name msgraph-e2e
helm install crossplane crossplane-stable/crossplane \
  -n crossplane-system --create-namespace --version 2.3.3 --wait

kubectl apply -f example/e2e/function.yaml
kubectl wait function.pkg.crossplane.io/function-msgraph --for=condition=Healthy --timeout=180s

kubectl apply -f example/definition.yaml
kubectl apply -f example/e2e/composition.yaml
kubectl apply -f example/secrets/azure-creds.yaml   # populate locally; keep real values out of commits
kubectl apply -f example/e2e/xr.yaml
```

## Verify

```shell
# status carries the results plus a lastQueryTime element
kubectl get xr msgraph-query-interval-e2e -o jsonpath='{.status.validatedUsers}' | jq

# within the interval, reconciles skip (condition is set on the XR)
kubectl get xr msgraph-query-interval-e2e -o jsonpath='{.status.conditions}' | jq   # FunctionSkip/IntervalLimit

# count real Graph calls vs skips in the function logs
POD=$(kubectl get pods -n crossplane-system -o name | grep msgraph)
kubectl logs -n crossplane-system "$POD" | grep -c 'Query Type'      # real queries
kubectl logs -n crossplane-system "$POD" | grep -c 'interval limit'  # skips
```

Force reconciles with `kubectl annotate xr msgraph-query-interval-e2e poke=$(date +%s) --overwrite`.
Poking repeatedly inside the 2m window leaves the query count flat; after 2m
elapses the next reconcile re-queries and `lastQueryTime` advances.

## Notes

- `queryInterval` is effective only in Composition mode with a `status.` target.
- The example XRD resolves to `LegacyCluster` scope under Crossplane v2, so the XR
  is cluster-scoped (no namespace).
- Teardown: `kind delete cluster --name msgraph-e2e`.
