package vsphere

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	apifixtures "github.com/openshift/hypershift/api/fixtures"
	"github.com/openshift/hypershift/cmd/cluster/core"
	"github.com/openshift/hypershift/support/infraid"
)

const (
	NodePortServicePublishingStrategy = "NodePort"
	IngressServicePublishingStrategy  = "Ingress"
)

func NewCreateCommand(opts *core.CreateOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "vsphere",
		Short:        "Creates basic functional HostedCluster resources for vSphere platform",
		SilenceUsage: true,
	}

	opts.VSpherePlatform = core.VSpherePlatformCreateOptions{
		ServicePublishingStrategy: IngressServicePublishingStrategy,
		APIServerAddress:          "",
		Memory:                    "4Gi",
		Cores:                     2,
		ContainerDiskImage:        "",
		RootVolumeSize:            16,
		InfraKubeConfigFile:       "",
	}

	cmd.Flags().StringVar(&opts.VSpherePlatform.APIServerAddress, "api-server-address", opts.VSpherePlatform.APIServerAddress, "The API server address that should be used for components outside the control plane")
	cmd.Flags().StringVar(&opts.VSpherePlatform.Memory, "memory", opts.VSpherePlatform.Memory, "The amount of memory which is visible inside the Guest OS (type BinarySI, e.g. 5Gi, 100Mi)")
	cmd.Flags().Uint32Var(&opts.VSpherePlatform.Cores, "cores", opts.VSpherePlatform.Cores, "The number of cores inside the vmi, Must be a value greater than or equal to 1")
	cmd.Flags().StringVar(&opts.VSpherePlatform.RootVolumeStorageClass, "root-volume-storage-class", opts.VSpherePlatform.RootVolumeStorageClass, "The storage class to use for machines in the NodePool")
	cmd.Flags().Uint32Var(&opts.VSpherePlatform.RootVolumeSize, "root-volume-size", opts.VSpherePlatform.RootVolumeSize, "The size of the root volume for machines in the NodePool in Gi")
	cmd.Flags().StringVar(&opts.VSpherePlatform.RootVolumeAccessModes, "root-volume-access-modes", opts.VSpherePlatform.RootVolumeAccessModes, "The access modes of the root volume to use for machines in the NodePool (comma-delimited list)")
	cmd.Flags().StringVar(&opts.VSpherePlatform.ContainerDiskImage, "containerdisk", opts.VSpherePlatform.ContainerDiskImage, "A reference to docker image with the embedded disk to be used to create the machines")
	cmd.Flags().StringVar(&opts.VSpherePlatform.ServicePublishingStrategy, "service-publishing-strategy", opts.VSpherePlatform.ServicePublishingStrategy, fmt.Sprintf("Define how to expose the cluster services. Supported options: %s (Use LoadBalancer and Route to expose services), %s (Select a random node to expose service access through)", IngressServicePublishingStrategy, NodePortServicePublishingStrategy))
	cmd.Flags().StringVar(&opts.VSpherePlatform.InfraKubeConfigFile, "infra-kubeconfig-file", opts.VSpherePlatform.InfraKubeConfigFile, "Path to a kubeconfig file of an external infra cluster to be used to create the guest clusters nodes onto")
	cmd.Flags().StringVar(&opts.VSpherePlatform.InfraNamespace, "infra-namespace", opts.VSpherePlatform.InfraNamespace, "The namespace in the external infra cluster that is used to host the vsphere virtual machines. The namespace must exist prior to creating the HostedCluster")

	cmd.MarkPersistentFlagRequired("pull-secret")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if opts.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
		}

		if err := CreateCluster(ctx, opts); err != nil {
			opts.Log.Error(err, "Failed to create cluster")
			return err
		}
		return nil
	}

	return cmd
}

func CreateCluster(ctx context.Context, opts *core.CreateOptions) error {
	return core.CreateCluster(ctx, opts, applyPlatformSpecificsValues)
}

func applyPlatformSpecificsValues(ctx context.Context, exampleOptions *apifixtures.ExampleOptions, opts *core.CreateOptions) (err error) {
	if opts.VSpherePlatform.ServicePublishingStrategy != NodePortServicePublishingStrategy && opts.VSpherePlatform.ServicePublishingStrategy != IngressServicePublishingStrategy {
		return fmt.Errorf("service publish strategy %s is not supported, supported options: %s, %s", opts.VSpherePlatform.ServicePublishingStrategy, IngressServicePublishingStrategy, NodePortServicePublishingStrategy)
	}
	if opts.VSpherePlatform.ServicePublishingStrategy != NodePortServicePublishingStrategy && opts.VSpherePlatform.APIServerAddress != "" {
		return fmt.Errorf("external-api-server-address is supported only for NodePort service publishing strategy, service publishing strategy %s is used", opts.VSpherePlatform.ServicePublishingStrategy)
	}
	if opts.VSpherePlatform.APIServerAddress == "" && opts.VSpherePlatform.ServicePublishingStrategy == NodePortServicePublishingStrategy && !opts.Render {
		if opts.VSpherePlatform.APIServerAddress, err = core.GetAPIServerAddressByNode(ctx, opts.Log); err != nil {
			return err
		}
	}

	if opts.VSpherePlatform.Cores < 1 {
		return errors.New("the number of cores inside the machine must be a value greater than or equal to 1")
	}

	if opts.VSpherePlatform.RootVolumeSize < 8 {
		return fmt.Errorf("the root volume size [%d] must be greater than or equal to 8", opts.VSpherePlatform.RootVolumeSize)
	}

	infraID := opts.InfraID
	if len(infraID) == 0 {
		exampleOptions.InfraID = infraid.New(opts.Name)
	} else {
		exampleOptions.InfraID = infraID
	}

	var infraKubeConfigContents []byte
	infraKubeConfigFile := opts.VSpherePlatform.InfraKubeConfigFile
	if len(infraKubeConfigFile) > 0 {
		infraKubeConfigContents, err = os.ReadFile(infraKubeConfigFile)
		if err != nil {
			return fmt.Errorf("failed to read external infra cluster kubeconfig file: %w", err)
		}
	} else {
		infraKubeConfigContents = nil
	}

	if opts.VSpherePlatform.InfraKubeConfigFile == "" && opts.VSpherePlatform.InfraNamespace != "" {
		return fmt.Errorf("external infra cluster namespace was provided but a kubeconfig is missing")
	}

	if opts.VSpherePlatform.InfraNamespace == "" && opts.VSpherePlatform.InfraKubeConfigFile != "" {
		return fmt.Errorf("external infra cluster kubeconfig was provided but an infra namespace is missing")
	}

	exampleOptions.VSphere = &apifixtures.ExampleVSphereOptions{
		ServicePublishingStrategy: opts.VSpherePlatform.ServicePublishingStrategy,
		APIServerAddress:          opts.VSpherePlatform.APIServerAddress,
		Memory:                    opts.VSpherePlatform.Memory,
		Cores:                     opts.VSpherePlatform.Cores,
		Image:                     opts.VSpherePlatform.ContainerDiskImage,
		RootVolumeSize:            opts.VSpherePlatform.RootVolumeSize,
		RootVolumeStorageClass:    opts.VSpherePlatform.RootVolumeStorageClass,
		RootVolumeAccessModes:     opts.VSpherePlatform.RootVolumeAccessModes,
		InfraKubeConfig:           infraKubeConfigContents,
		InfraNamespace:            opts.VSpherePlatform.InfraNamespace,
	}

	if opts.BaseDomain != "" {
		exampleOptions.BaseDomain = opts.BaseDomain
	} else {
		exampleOptions.VSphere.BaseDomainPassthrough = true
	}

	return nil
}
