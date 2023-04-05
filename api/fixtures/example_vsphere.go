package fixtures

import (
	"fmt"
	"strings"

	hyperv1 "github.com/openshift/hypershift/api/v1beta1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
)

type ExampleVSphereOptions struct {
	ServicePublishingStrategy string
	APIServerAddress          string
	Memory                    string
	Cores                     uint32
	Image                     string
	RootVolumeSize            uint32
	RootVolumeStorageClass    string
	RootVolumeAccessModes     string
	BaseDomainPassthrough     bool
	InfraKubeConfig           []byte
	InfraNamespace            string
}

func ExampleVSphereTemplate(o *ExampleVSphereOptions) *hyperv1.VSphereNodePoolPlatform {
	var storageClassName *string
	var accessModesStr []string
	var accessModes []hyperv1.PersistentVolumeAccessMode
	volumeSize := apiresource.MustParse(fmt.Sprintf("%vGi", o.RootVolumeSize))

	if o.RootVolumeStorageClass != "" {
		storageClassName = &o.RootVolumeStorageClass
	}

	if o.RootVolumeAccessModes != "" {
		accessModesStr = strings.Split(o.RootVolumeAccessModes, ",")
		for _, ams := range accessModesStr {
			var am hyperv1.PersistentVolumeAccessMode
			am = hyperv1.PersistentVolumeAccessMode(ams)
			accessModes = append(accessModes, am)
		}
	}

	exampleTemplate := &hyperv1.VSphereNodePoolPlatform{
		RootVolume: &hyperv1.VSphereRootVolume{
			VSphereVolume: hyperv1.VSphereVolume{
				Type: hyperv1.VSphereVolumeTypePersistent,
				Persistent: &hyperv1.VSpherePersistentVolume{
					Size:         &volumeSize,
					StorageClass: storageClassName,
					AccessModes:  accessModes,
				},
			},
		},
		Compute: &hyperv1.VSphereCompute{},
	}

	if o.Memory != "" {
		memory := apiresource.MustParse(o.Memory)
		exampleTemplate.Compute.Memory = &memory
	}
	if o.Cores != 0 {
		exampleTemplate.Compute.Cores = &o.Cores
	}

	if o.Image != "" {
		exampleTemplate.RootVolume.Image = &hyperv1.VSphereDiskImage{
			ContainerDiskImage: &o.Image,
		}
	}

	return exampleTemplate
}
