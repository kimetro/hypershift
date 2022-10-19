package kubevirt

import (
	"fmt"

	hyperv1 "github.com/openshift/hypershift/api/v1alpha1"
	"github.com/openshift/hypershift/control-plane-operator/controllers/hostedcontrolplane/manifests"
	"github.com/openshift/hypershift/support/config"
	"github.com/openshift/hypershift/support/releaseinfo"
	"github.com/openshift/hypershift/support/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func ReconcileCloudConfig(cm *corev1.ConfigMap, hcp *hyperv1.HostedControlPlane) error {
	cfg := cloudConfig(hcp.Namespace)
	serializedCfg, err := cfg.serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize cloudconfig: %w", err)
	}

	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[CloudConfigKey] = string(serializedCfg)

	return nil
}

func ReconcileCCMServiceAccount(sa *corev1.ServiceAccount, ownerRef config.OwnerRef) error {
	ownerRef.ApplyTo(sa)
	return nil
}

func ReconcileCCMRole(role *rbacv1.Role, ownerRef config.OwnerRef) error {
	ownerRef.ApplyTo(role)
	role.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"kubevirt.io"},
			Resources: []string{"virtualmachines"},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{"kubevirt.io"},
			Resources: []string{"virtualmachineinstances"},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"update",
			},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"services"},
			Verbs:     []string{rbacv1.VerbAll},
		},
	}
	return nil
}

func ReconcileCCMRoleBinding(roleBinding *rbacv1.RoleBinding, ownerRef config.OwnerRef, sa *corev1.ServiceAccount, role *rbacv1.Role) error {
	ownerRef.ApplyTo(roleBinding)
	roleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}
	roleBinding.Subjects = []rbacv1.Subject{
		{
			Namespace: sa.Namespace,
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
		},
	}
	return nil
}

func ReconcileDeployment(deployment *appsv1.Deployment, hcp *hyperv1.HostedControlPlane, serviceAccountName string, releaseImage *releaseinfo.ReleaseImage) error {
	clusterName, ok := hcp.Labels["cluster.x-k8s.io/cluster-name"]
	if !ok {
		return fmt.Errorf("\"cluster.x-k8s.io/cluster-name\" label doesn't exist in HostedControlPlane")
	}
	deploymentConfig := newDeploymentConfig()
	deployment.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: ccmLabels(),
		},
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: ccmLabels(),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					util.BuildContainer(CCMContainer(), buildCCMContainer(clusterName, releaseImage)),
				},
				Volumes:            []corev1.Volume{},
				ServiceAccountName: serviceAccountName,
			},
		},
	}

	addVolumes(deployment)

	config.OwnerRefFrom(hcp).ApplyTo(deployment)
	deploymentConfig.ApplyTo(deployment)
	return nil
}

func addVolumes(deployment *appsv1.Deployment) {

	deployment.Spec.Template.Spec.Volumes = append(
		deployment.Spec.Template.Spec.Volumes,
		util.BuildVolume(ccmVolumeKubeconfig(), buildCCMVolumeKubeconfig),
	)
	deployment.Spec.Template.Spec.Volumes = append(
		deployment.Spec.Template.Spec.Volumes,
		util.BuildVolume(ccmCloudConfig(), buildCCMCloudConfig),
	)
}

func podVolumeMounts() util.PodVolumeMounts {
	return util.PodVolumeMounts{
		CCMContainer().Name: util.ContainerVolumeMounts{
			ccmVolumeKubeconfig().Name: "/etc/kubernetes/kubeconfig",
			ccmCloudConfig().Name:      "/etc/cloud",
		},
	}
}

func buildCCMContainer(clusterName string, releaseImage *releaseinfo.ReleaseImage) func(c *corev1.Container) {
	return func(c *corev1.Container) {
		c.Image = releaseImage.ComponentImages()["kubevirt-cloud-controller-manager"]
		c.ImagePullPolicy = corev1.PullIfNotPresent
		c.Command = []string{"/bin/kubevirt-cloud-controller-manager"}
		c.Args = []string{
			"--cloud-provider=kubevirt",
			"--cloud-config=/etc/cloud/cloud-config",
			"--kubeconfig=/etc/kubernetes/kubeconfig/kubeconfig",
			"--authentication-skip-lookup",
			"--cluster-name", clusterName,
		}
		c.VolumeMounts = podVolumeMounts().ContainerMounts(c.Name)
	}
}

func buildCCMVolumeKubeconfig(v *corev1.Volume) {
	v.Secret = &corev1.SecretVolumeSource{
		SecretName:  manifests.KASServiceKubeconfigSecret("").Name,
		DefaultMode: pointer.Int32Ptr(0640),
	}
}

func buildCCMCloudConfig(v *corev1.Volume) {
	v.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: CCMConfigMap("").Name,
		},
	}
}

func newDeploymentConfig() config.DeploymentConfig {
	result := config.DeploymentConfig{}
	result.Resources = config.ResourcesSpec{
		CCMContainer().Name: {
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("60Mi"),
				corev1.ResourceCPU:    resource.MustParse("75m"),
			},
		},
	}
	result.AdditionalLabels = additionalLabels()
	result.Scheduling.PriorityClass = config.DefaultPriorityClass

	result.Replicas = 1

	return result
}

func ccmLabels() map[string]string {
	return map[string]string{
		"app": "cloud-controller-manager",
	}
}

func additionalLabels() map[string]string {
	return map[string]string{
		hyperv1.ControlPlaneComponent: "cloud-controller-manager",
	}
}
