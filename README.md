# function-azresourcegraph
[![CI](https://github.com/UpboundCare//function-azresourcegraph/actions/workflows/ci.yml/badge.svg)](https://github.com/UpboundCare/function-azresourcegraph/actions/workflows/ci.yml)

A function to query [Azure Resource Graph][azresourcegraph]

## Usage

See [examples][examples]

Example pipeline step:

```yaml
  pipeline:
  - step: query-azresourcegraph
    functionRef:
      name: function-azresourcegraph
    input:
      apiVersion: azresourcegraph.fn.crossplane.io/v1alpha1
      kind: Input
      query: "Resources | project name, location, type, id| where type =~ 'Microsoft.Compute/virtualMachines' | order by name desc"
    credentials:
      - name: azure-creds
        source: Secret
        secretRef:
          namespace: upbound-system
          name: azure-account-creds
```

The Azure Credentials Secret structure is fully compatible with the standard
[Azure Official Provider][azop]

Example XR status after e2e query:

```yaml
apiVersion: example.crossplane.io/v1
kind: XR
metadata:
...
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
```


[azresourcegraph]: https://learn.microsoft.com/en-us/azure/governance/resource-graph/
[azop]: https://marketplace.upbound.io/providers/upbound/provider-family-azure/latest
[examples]: ./example
