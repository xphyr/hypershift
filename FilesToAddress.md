The structure used to create VMs for KubeVirt leverages CAPI and is the perfect framework for me to follow (I think)
So I created a "vsphere" structure everywhere there is a kubevirt structure ... copy/pasting and then changing names
This is the list of everything for me to review/update to make this work

## API Files:

| File Name                          | CopyPaste Cleanup |
| ---------------------------------- | :---------------: |
| api/fixtures/example.go            |         0         |
| api/fixtures/example_vsphere.go    |         0         |
| api/v1beta1/hostedcluster_types.go |         0         |
| api/v1beta1/nodepool_types.go      |         0         |

## CMD Files:

| File Name                      | CopyPaste Cleanup |
| ------------------------------ | :---------------: |
| cmd/cluster/core/create.go     |         0         |
| cmd/cluster/vsphere/create.go  |         0         |
| cmd/cluster/vsphere/destroy.go |         0         |
| cmd/nodepool/vsphere/create.go |         0         |

## HyperShift Controllers Platform:

| File Name                                                                               | CopyPaste Cleanup |
| --------------------------------------------------------------------------------------- | :---------------: |
| hypershift-operator/controllers/hostedcluster/internal/platform/vsphere/OWNERS          |         X         |
| hypershift-operator/controllers/hostedcluster/internal/platform/vsphere/vsphere.go      |         0         |
| hypershift-operator/controllers/hostedcluster/internal/platform/vsphere/vsphere_test.go |         0         |

## HypserShift Controllers NodePool:

| File Name                                                        | CopyPaste Cleanup |
| ---------------------------------------------------------------- | :---------------: |
| hypershift-operator/controllers/nodepool/vsphere/OWNERS          |         X         |
| hypershift-operator/controllers/nodepool/vsphere/vsphere.go      |         0         |
| hypershift-operator/controllers/nodepool/vsphere/vsphere_test.go |         0         |

### TODO: 
* Find the right image to add as default in `hypershift-operator/controllers/hostedcluster/internal/platform/vsphere/vsphere.go`


## OTHER:

Only thing needed here was to add another env variable for the vsphere capi override

| File Name                 | CopyPaste Cleanup |
| ------------------------- | :---------------: |
| support/images/envvars.go |         X         |


