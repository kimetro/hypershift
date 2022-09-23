package olm

import (
	"github.com/openshift/hypershift/support/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

var (
	olmCollectProfilesConfigMap   = MustConfigMap("assets/olm-collect-profiles.configmap.yaml")
	olmCollectProfilesCronJob     = MustCronJob("assets/olm-collect-profiles.cronjob.yaml")
	olmCollectProfilesRole        = MustRole("assets/olm-collect-profiles.role.yaml")
	olmCollectProfilesRoleBinding = MustRoleBinding("assets/olm-collect-profiles.rolebinding.yaml")
	olmCollectProfilesSecret      = MustSecret("assets/olm-collect-profiles.secret.yaml")
)

func ReconcileCollectProfilesCronJob(cronJob *batchv1.CronJob, ownerRef config.OwnerRef, olmImage, namespace string) {
	ownerRef.ApplyTo(cronJob)
	cronJob.Spec = olmCollectProfilesCronJob.DeepCopy().Spec
	cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = olmImage
	for i, arg := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args {
		if arg == "OLM_NAMESPACE" {
			cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args[i] = namespace
		}
	}
	cronJob.Spec.Schedule = generateModularDailyCronSchedule([]byte(cronJob.Namespace))
}

func ReconcileCollectProfilesConfigMap(configMap *corev1.ConfigMap, ownerRef config.OwnerRef) {
	ownerRef.ApplyTo(configMap)
	configMap.Data = olmCollectProfilesConfigMap.DeepCopy().Data
}

func ReconcileCollectProfilesRole(role *rbacv1.Role, ownerRef config.OwnerRef) {
	ownerRef.ApplyTo(role)
	role.Rules = olmCollectProfilesRole.DeepCopy().Rules
}

func ReconcileCollectProfilesRoleBinding(roleBinding *rbacv1.RoleBinding, ownerRef config.OwnerRef) {
	ownerRef.ApplyTo(roleBinding)
	roleBinding.RoleRef = olmCollectProfilesRoleBinding.DeepCopy().RoleRef
	roleBinding.Subjects = olmCollectProfilesRoleBinding.DeepCopy().Subjects
}

func ReconcileCollectProfilesSecret(secret *corev1.Secret, ownerRef config.OwnerRef) {
	ownerRef.ApplyTo(secret)
	secret.Type = olmCollectProfilesSecret.Type
	secret.Data = olmCollectProfilesSecret.DeepCopy().Data
}

func ReconcileCollectProfilesServiceAccount(serviceAccount *corev1.ServiceAccount, ownerRef config.OwnerRef) {
	ownerRef.ApplyTo(serviceAccount)
}
