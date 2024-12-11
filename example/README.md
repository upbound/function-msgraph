# Example manifests

You can run your function locally and test it using `crossplane beta render`
with these example manifests.

```shell
# Run the function locally
$ go run . --insecure --debug
```

```shell
# Then, in another terminal, call it with these example manifests
$ crossplane render example/xr.yaml example/composition.yaml example/functions-dev.yaml --function-credentials=example/secrets/azure-creds.yaml -r
---
apiVersion: example.crossplane.io/v1
kind: XR
metadata:
  name: example-xr
status:
  azResourceGraphQueryResult:
  - id: /subscriptions/f403a412-959c-4214-8c4d-ad5598f149cc/resourceGroups/us-vm-zxqnj-s2jdb/providers/Microsoft.Compute/virtualMachines/us-vm-zxqnj-2h59v
    location: centralus
    name: us-vm-zxqnj-2h59v
    type: microsoft.compute/virtualmachines
  - id: /subscriptions/f403a412-959c-4214-8c4d-ad5598f149cc/resourceGroups/us-vm-lzbpt-tdv2h/providers/Microsoft.Compute/virtualMachines/us-vm-lzbpt-fgcds
    location: centralus
    name: us-vm-lzbpt-fgcds
    type: microsoft.compute/virtualmachines
  - id: /subscriptions/f403a412-959c-4214-8c4d-ad5598f149cc/resourceGroups/us-vac-dr27h-ttsq5/providers/Microsoft.Compute/virtualMachines/us-vac-dr27h-t7dhd
    location: centralus
    name: us-vac-dr27h-t7dhd
    type: microsoft.compute/virtualmachines
  - id: /subscriptions/f403a412-959c-4214-8c4d-ad5598f149cc/resourceGroups/my-vm-mm59z/providers/Microsoft.Compute/virtualMachines/my-vm-jm8g2
    location: swedencentral
    name: my-vm-jm8g2
    type: microsoft.compute/virtualmachines
  - id: /subscriptions/f403a412-959c-4214-8c4d-ad5598f149cc/resourceGroups/javid-labs/providers/Microsoft.Compute/virtualMachines/devstack-test
    location: westus2
    name: devstack-test
    type: microsoft.compute/virtualmachines
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    reason: Available
    status: "True"
    type: Ready
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: 'Query: "Resources | project name, location, type, id| where type =~ ''Microsoft.Compute/virtualMachines''
  | order by name desc"'
severity: SEVERITY_NORMAL
step: query-azresourcegraph
```
