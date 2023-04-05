package vsphere

import (
	"context"
	"fmt"
	"os"

	hyperv1 "github.com/openshift/hypershift/api/v1beta1"
	"github.com/openshift/hypershift/support/images"
	"github.com/openshift/hypershift/support/upsert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sutilspointer "k8s.io/utils/pointer"
	capivsphere "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	hostedClusterAnnotation = "hypershift.openshift.io/cluster"
	imageCAPK               = "registry.ci.openshift.org/ocp/4.14:cluster-api-provider-vsphere"
)

type VSphere struct{}

func (p VSphere) ReconcileCAPIInfraCR(ctx context.Context, c client.Client, createOrUpdate upsert.CreateOrUpdateFN,
	hcluster *hyperv1.HostedCluster,
	controlPlaneNamespace string, _ hyperv1.APIEndpoint) (client.Object, error) {
	vsphereCluster := &capivsphere.VSphereCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: controlPlaneNamespace,
			Name:      hcluster.Spec.InfraID,
		},
	}
	kvPlatform := hcluster.Spec.Platform.VSphere
	if kvPlatform != nil && kvPlatform.Credentials != nil {
		var infraClusterSecretRef = &corev1.ObjectReference{
			Name:      hyperv1.KubeVirtInfraCredentialsSecretName,
			Namespace: controlPlaneNamespace,
			Kind:      "Secret",
		}
		vsphereCluster.Spec.InfraClusterSecretRef = infraClusterSecretRef
	}
	if _, err := createOrUpdate(ctx, c, vsphereCluster, func() error {
		reconcileVSphereCluster(vsphereCluster, hcluster)
		return nil
	}); err != nil {
		return nil, err
	}

	return vsphereCluster, nil
}

func reconcileVSphereCluster(vsphereCluster *capivsphere.VSphereCluster, hcluster *hyperv1.HostedCluster) {
	// We only create this resource once and then let CAPI own it
	vsphereCluster.Annotations = map[string]string{
		hostedClusterAnnotation:    client.ObjectKeyFromObject(hcluster).String(),
		capiv1.ManagedByAnnotation: "external",
	}
	// Set the values for upper level controller
	vsphereCluster.Status.Ready = true
}

func (p VSphere) CAPIProviderDeploymentSpec(hcluster *hyperv1.HostedCluster, _ *hyperv1.HostedControlPlane) (*appsv1.DeploymentSpec, error) {
	providerImage := imageCAPK
	if envImage := os.Getenv(images.VSphereCAPIProviderEnvVar); len(envImage) > 0 {
		providerImage = envImage
	}
	if override, ok := hcluster.Annotations[hyperv1.ClusterAPIKubeVirtProviderImage]; ok {
		providerImage = override
	}
	defaultMode := int32(0640)
	return &appsv1.DeploymentSpec{
		Replicas: k8sutilspointer.Int32Ptr(1),
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				TerminationGracePeriodSeconds: k8sutilspointer.Int64Ptr(10),
				Tolerations: []corev1.Toleration{
					{
						Key:    "node-role.kubernetes.io/master",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "capi-webhooks-tls",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								DefaultMode: &defaultMode,
								SecretName:  "capi-webhooks-tls",
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:            "manager",
						Image:           providerImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("100Mi"),
								corev1.ResourceCPU:    resource.MustParse("10m"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "capi-webhooks-tls",
								ReadOnly:  true,
								MountPath: "/tmp/k8s-webhook-server/serving-certs",
							},
						},
						Env: []corev1.EnvVar{
							{
								Name: "MY_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
						},
						Command: []string{"/manager"},
						Args: []string{
							"--namespace", "$(MY_NAMESPACE)",
							"--alsologtostderr",
							"--v=4",
							"--leader-elect=true",
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "healthz",
								ContainerPort: 9440,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromString("healthz"),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/readyz",
									Port: intstr.FromString("healthz"),
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

func (p VSphere) ReconcileCredentials(ctx context.Context, c client.Client, createOrUpdate upsert.CreateOrUpdateFN,
	hcluster *hyperv1.HostedCluster,
	controlPlaneNamespace string) error {

	// If external infra cluster kubeconfig has been provided, copy the secret from the "clusters" to the hosted control plane namespace
	// with the predictable name "vsphere-infra-credentials"
	kvPlatform := hcluster.Spec.Platform.VSphere
	if kvPlatform == nil || kvPlatform.Credentials == nil {
		return nil
	}

	var sourceSecret corev1.Secret
	secretName := client.ObjectKey{Namespace: hcluster.Namespace, Name: hcluster.Spec.Platform.VSphere.Credentials.InfraKubeConfigSecret.Name}
	if err := c.Get(ctx, secretName, &sourceSecret); err != nil {
		return fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}
	targetSecret := credentialsSecret(controlPlaneNamespace)
	_, err := createOrUpdate(ctx, c, targetSecret, func() error {
		if targetSecret.Data == nil {
			targetSecret.Data = map[string][]byte{}
		}
		for k, v := range sourceSecret.Data {
			targetSecret.Data[k] = v
		}
		return nil
	})
	return err
}

func (VSphere) ReconcileSecretEncryption(ctx context.Context, c client.Client, createOrUpdate upsert.CreateOrUpdateFN,
	hcluster *hyperv1.HostedCluster,
	controlPlaneNamespace string) error {
	return nil
}

func (VSphere) CAPIProviderPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"services"},
			Verbs:     []string{"*"},
		},
		{
			APIGroups: []string{"vsphere.io"},
			Resources: []string{"virtualmachineinstances", "virtualmachines"},
			Verbs:     []string{"*"},
		},
	}
}

func credentialsSecret(hcpNamespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hyperv1.KubeVirtInfraCredentialsSecretName,
			Namespace: hcpNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func (VSphere) DeleteCredentials(ctx context.Context, c client.Client, hcluster *hyperv1.HostedCluster, controlPlaneNamespace string) error {
	return nil
}
