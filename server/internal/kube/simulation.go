package kube

import (
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func NewSimulationFactory() *Factory {
	kubeClient := kubefake.NewSimpleClientset(simulationObjects()...)
	if discovery, ok := kubeClient.Discovery().(*fakediscovery.FakeDiscovery); ok {
		discovery.FakedServerVersion = &version.Info{GitVersion: "v1.35.3-sim"}
	}

	return &Factory{
		configPath: "/virtual/kubesafe/simulation-kubeconfig",
		baseConfig: &rest.Config{Host: "https://kubesafe-simulation.local"},
		mode:       "simulation",
		simKube:    kubeClient,
		simMetrics: metricsfake.NewSimpleClientset(simulationMetricsObjects()...),
		rawConfig: clientcmdapiConfig{
			CurrentContext: "kubesafe-simulation",
			AuthInfoName:   "simulation-user",
		},
	}
}

func simulationObjects() []runtime.Object {
	now := time.Now()
	older := metav1.NewTime(now.Add(-6 * time.Hour))
	recent := metav1.NewTime(now.Add(-18 * time.Minute))
	replicas := int32(3)
	badReplicas := int32(2)
	crashReplicas := int32(1)
	probeReplicas := int32(1)
	oomReplicas := int32(1)
	storageClass := "local-path"
	missingStorageClass := "fast-ssd"
	controller := true

	return []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default", CreationTimestamp: older}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "demo-workloads", CreationTimestamp: older}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "demo-broken", CreationTimestamp: older}},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "sim-master",
				CreationTimestamp: older,
				Labels: map[string]string{
					"kubernetes.io/hostname":                "sim-master",
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.10.0.10"}},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("110"),
				},
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("8Gi"),
					corev1.ResourcePods:   resource.MustParse("110"),
				},
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastTransitionTime: older}},
				NodeInfo: corev1.NodeSystemInfo{
					KubeletVersion:          "v1.35.3-sim",
					OSImage:                 "Kubejojo Simulation Linux",
					KernelVersion:           "6.8.0-sim",
					ContainerRuntimeVersion: "containerd://2.2.1",
					Architecture:            "arm64",
					OperatingSystem:         "linux",
					MachineID:               "sim-master",
					SystemUUID:              "sim-master",
					BootID:                  "sim-master",
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "sim-worker-1",
				CreationTimestamp: older,
				Labels:            map[string]string{"kubernetes.io/hostname": "sim-worker-1"},
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.10.0.11"}},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
					corev1.ResourcePods:   resource.MustParse("110"),
				},
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
					corev1.ResourcePods:   resource.MustParse("110"),
				},
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastTransitionTime: older}},
				NodeInfo: corev1.NodeSystemInfo{
					KubeletVersion:          "v1.35.3-sim",
					OSImage:                 "Kubejojo Simulation Linux",
					KernelVersion:           "6.8.0-sim",
					ContainerRuntimeVersion: "containerd://2.2.1",
					Architecture:            "arm64",
					OperatingSystem:         "linux",
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "web-portal", Namespace: "demo-workloads", UID: types.UID("deploy-web-portal"), CreationTimestamp: older, Labels: appLabels("web-portal")},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web-portal"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: appLabels("web-portal")},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{
						Name:  "nginx",
						Image: "nginx:1.27-alpine",
						Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 80}},
					}}},
				},
			},
			Status: appsv1.DeploymentStatus{Replicas: replicas, UpdatedReplicas: replicas, ReadyReplicas: replicas, AvailableReplicas: replicas},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "checkout-api", Namespace: "demo-broken", UID: types.UID("deploy-checkout-api"), CreationTimestamp: older, Labels: appLabels("checkout-api")},
			Spec: appsv1.DeploymentSpec{
				Replicas: &badReplicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "checkout-api"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: appLabels("checkout-api")},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{
						Name:  "api",
						Image: "registry.invalid.local/checkout-api:v1",
						Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}},
					}}},
				},
			},
			Status: appsv1.DeploymentStatus{Replicas: badReplicas, UpdatedReplicas: badReplicas, ReadyReplicas: 0, AvailableReplicas: 0, UnavailableReplicas: badReplicas},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "payment-worker", Namespace: "demo-broken", UID: types.UID("deploy-payment-worker"), CreationTimestamp: older, Labels: appLabels("payment-worker")},
			Spec: appsv1.DeploymentSpec{
				Replicas: &crashReplicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "payment-worker"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: appLabels("payment-worker")},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{
						Name:    "worker",
						Image:   "busybox:1.36",
						Command: []string{"sh", "-c"},
						Args:    []string{"echo starting payment worker; exit 1"},
					}}},
				},
			},
			Status: appsv1.DeploymentStatus{Replicas: crashReplicas, UpdatedReplicas: crashReplicas, ReadyReplicas: 0, AvailableReplicas: 0, UnavailableReplicas: crashReplicas},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "profile-api", Namespace: "demo-broken", UID: types.UID("deploy-profile-api"), CreationTimestamp: older, Labels: appLabels("profile-api")},
			Spec: appsv1.DeploymentSpec{
				Replicas: &probeReplicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "profile-api"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: appLabels("profile-api")},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{
						Name:  "api",
						Image: "nginx:1.27",
						Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}},
						ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
							Path: "/fail",
							Port: intstr.FromInt32(8080),
						}}},
					}}},
				},
			},
			Status: appsv1.DeploymentStatus{Replicas: probeReplicas, UpdatedReplicas: probeReplicas, ReadyReplicas: 0, AvailableReplicas: 0, UnavailableReplicas: probeReplicas},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "analytics-batch", Namespace: "demo-broken", UID: types.UID("deploy-analytics-batch"), CreationTimestamp: older, Labels: appLabels("analytics-batch")},
			Spec: appsv1.DeploymentSpec{
				Replicas: &oomReplicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "analytics-batch"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: appLabels("analytics-batch")},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{
						Name:  "batch",
						Image: "busybox:1.36",
						Args:  []string{"simulate-oom"},
						Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						}},
					}}},
				},
			},
			Status: appsv1.DeploymentStatus{Replicas: oomReplicas, UpdatedReplicas: oomReplicas, ReadyReplicas: 0, AvailableReplicas: 0, UnavailableReplicas: oomReplicas},
		},
		replicaSet("web-portal-6d8f7f9c9c", "demo-workloads", "rs-web-portal", "deploy-web-portal", replicas, replicas, controller),
		replicaSet("checkout-api-777b4dbdb8", "demo-broken", "rs-checkout-api", "deploy-checkout-api", badReplicas, 0, controller),
		replicaSetForApp("payment-worker-5c7d8f9d6d", "payment-worker", "demo-broken", "rs-payment-worker", "deploy-payment-worker", crashReplicas, 0, controller),
		replicaSetForApp("profile-api-6b79cfd8b8", "profile-api", "demo-broken", "rs-profile-api", "deploy-profile-api", probeReplicas, 0, controller),
		replicaSetForApp("analytics-batch-764dbdd7cc", "analytics-batch", "demo-broken", "rs-analytics-batch", "deploy-analytics-batch", oomReplicas, 0, controller),
		runningPod("web-portal-6d8f7f9c9c-a1b2c", "demo-workloads", "pod-web-a", "sim-worker-1", "10.244.1.21", "rs-web-portal", older),
		runningPod("web-portal-6d8f7f9c9c-d4e5f", "demo-workloads", "pod-web-b", "sim-worker-1", "10.244.1.22", "rs-web-portal", older),
		runningPod("web-portal-6d8f7f9c9c-g7h8i", "demo-workloads", "pod-web-c", "sim-master", "10.244.0.18", "rs-web-portal", recent),
		imagePullPod("checkout-api-777b4dbdb8-bad01", "demo-broken", "pod-checkout-a", "sim-worker-1", "rs-checkout-api", recent),
		imagePullPod("checkout-api-777b4dbdb8-bad02", "demo-broken", "pod-checkout-b", "sim-master", "rs-checkout-api", recent),
		crashLoopPod("payment-worker-5c7d8f9d6d-crash01", "demo-broken", "pod-payment-worker-a", "sim-worker-1", "rs-payment-worker", recent),
		probeFailurePod("profile-api-6b79cfd8b8-probe01", "demo-broken", "pod-profile-api-a", "sim-worker-1", "10.244.2.41", "rs-profile-api", recent),
		oomKilledPod("analytics-batch-764dbdd7cc-oom01", "demo-broken", "pod-analytics-batch-a", "sim-master", "rs-analytics-batch", recent),
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "web-portal", Namespace: "demo-workloads", UID: types.UID("svc-web-portal"), CreationTimestamp: older, Labels: appLabels("web-portal")},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.96.10.20",
				Selector:  map[string]string{"app": "web-portal"},
				Ports:     []corev1.ServicePort{{Name: "http", Port: 80}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "web-shadow", Namespace: "demo-workloads", UID: types.UID("svc-web-shadow"), CreationTimestamp: older, Labels: appLabels("web-shadow")},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.96.10.21",
				Selector:  map[string]string{"app": "web-portal-v2"},
				Ports:     []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt32(80)}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "checkout-api", Namespace: "demo-broken", UID: types.UID("svc-checkout-api"), CreationTimestamp: older, Labels: appLabels("checkout-api")},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.96.10.30",
				Selector:  map[string]string{"app": "checkout-api-v2"},
				Ports:     []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt32(8080)}},
			},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "web-portal", Namespace: "demo-workloads", UID: types.UID("ing-web-portal"), CreationTimestamp: older, Labels: appLabels("web-portal")},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					Host: "web.sim.local",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/",
								PathType: pathTypePtr(networkingv1.PathTypePrefix),
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "web-portal",
										Port: networkingv1.ServiceBackendPort{Number: 80},
									},
								},
							}},
						},
					},
				}},
			},
			Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "10.10.0.240"}}}},
		},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: storageClass, CreationTimestamp: older}, Provisioner: "rancher.io/local-path"},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "web-cache", Namespace: "demo-workloads", UID: types.UID("pvc-web-cache"), CreationTimestamp: older},
			Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClass, AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound, Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "report-cache", Namespace: "demo-broken", UID: types.UID("pvc-report-cache"), CreationTimestamp: recent},
			Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: &missingStorageClass, AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")}}},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
		},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "web-config", Namespace: "demo-workloads", CreationTimestamp: older}, Data: map[string]string{"LOG_LEVEL": "info", "FEATURE_FLAG": "simulation"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "web-secret", Namespace: "demo-workloads", CreationTimestamp: older}, Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"API_TOKEN": []byte("simulation-token")}},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "pod-reader", Namespace: "demo-workloads", CreationTimestamp: older}, Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods", "pods/log"}, Verbs: []string{"get", "list"}}}},
		&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "pod-reader-binding", Namespace: "demo-workloads", CreationTimestamp: older}, Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "default", Namespace: "demo-workloads"}}, RoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "pod-reader"}},
		warningEvent("demo-broken", "checkout-api-777b4dbdb8-bad01", "Pod", "Failed", "Failed to pull image \"registry.invalid.local/checkout-api:v1\"", 8, recent),
		warningEvent("demo-broken", "checkout-api", "Deployment", "ProgressDeadlineExceeded", "Deployment has minimum availability issue in simulation", 3, recent),
		warningEvent("demo-broken", "payment-worker-5c7d8f9d6d-crash01", "Pod", "BackOff", "Back-off restarting failed container worker", 12, recent),
		warningEvent("demo-broken", "payment-worker", "Deployment", "ProgressDeadlineExceeded", "Deployment is waiting for payment-worker container to stay alive", 4, recent),
		warningEvent("demo-broken", "profile-api-6b79cfd8b8-probe01", "Pod", "Unhealthy", "Readiness probe failed: HTTP probe failed with statuscode: 500", 9, recent),
		warningEvent("demo-broken", "profile-api", "Deployment", "MinimumReplicasUnavailable", "Deployment has no ready replicas because readiness probe fails", 4, recent),
		warningEvent("demo-broken", "analytics-batch-764dbdd7cc-oom01", "Pod", "OOMKilled", "Container batch was terminated because it exceeded memory limit 32Mi", 5, recent),
		warningEvent("demo-broken", "analytics-batch", "Deployment", "ProgressDeadlineExceeded", "Deployment is waiting for analytics batch pod after OOMKilled restarts", 3, recent),
		warningEvent("demo-broken", "report-cache", "PersistentVolumeClaim", "ProvisioningFailed", "storageclass.storage.k8s.io \"fast-ssd\" not found", 6, recent),
	}
}

func simulationMetricsObjects() []runtime.Object {
	now := metav1.NewTime(time.Now())
	return []runtime.Object{
		&metricsv1beta1.NodeMetrics{ObjectMeta: metav1.ObjectMeta{Name: "sim-master", CreationTimestamp: now}, Usage: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("650m"), corev1.ResourceMemory: resource.MustParse("1800Mi")}},
		&metricsv1beta1.NodeMetrics{ObjectMeta: metav1.ObjectMeta{Name: "sim-worker-1", CreationTimestamp: now}, Usage: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1250m"), corev1.ResourceMemory: resource.MustParse("3200Mi")}},
		&metricsv1beta1.PodMetrics{ObjectMeta: metav1.ObjectMeta{Name: "web-portal-6d8f7f9c9c-a1b2c", Namespace: "demo-workloads", CreationTimestamp: now}, Containers: []metricsv1beta1.ContainerMetrics{{Name: "nginx", Usage: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("80m"), corev1.ResourceMemory: resource.MustParse("92Mi")}}}},
		&metricsv1beta1.PodMetrics{ObjectMeta: metav1.ObjectMeta{Name: "web-portal-6d8f7f9c9c-d4e5f", Namespace: "demo-workloads", CreationTimestamp: now}, Containers: []metricsv1beta1.ContainerMetrics{{Name: "nginx", Usage: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("76m"), corev1.ResourceMemory: resource.MustParse("88Mi")}}}},
	}
}

func appLabels(name string) map[string]string {
	return map[string]string{"app": name, "app.kubernetes.io/name": name, "app.kubernetes.io/part-of": "kubesafe-simulation"}
}

func replicaSet(name, namespace, uid, ownerUID string, replicas, ready int32, controller bool) *appsv1.ReplicaSet {
	appName := strings.TrimSuffix(strings.TrimSuffix(name, "-6d8f7f9c9c"), "-777b4dbdb8")
	return replicaSetForApp(name, appName, namespace, uid, ownerUID, replicas, ready, controller)
}

func replicaSetForApp(name, appName, namespace, uid, ownerUID string, replicas, ready int32, controller bool) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: types.UID(uid), CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Hour)), Labels: appLabels(appName), OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: appName, UID: types.UID(ownerUID), Controller: &controller}}},
		Spec:       appsv1.ReplicaSetSpec{Replicas: &replicas, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": appName}}},
		Status:     appsv1.ReplicaSetStatus{Replicas: replicas, ReadyReplicas: ready, AvailableReplicas: ready, FullyLabeledReplicas: replicas},
	}
}

func runningPod(name, namespace, uid, nodeName, podIP, ownerUID string, created metav1.Time) *corev1.Pod {
	controller := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			UID:               types.UID(uid),
			CreationTimestamp: created,
			Labels:            appLabels("web-portal"),
			OwnerReferences:   []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "web-portal-6d8f7f9c9c", UID: types.UID(ownerUID), Controller: &controller}},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name:  "nginx",
				Image: "nginx:1.27-alpine",
				Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 80}},
			}},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			PodIP:    podIP,
			QOSClass: corev1.PodQOSBurstable,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
			},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "nginx",
				Image:        "nginx:1.27-alpine",
				Ready:        true,
				RestartCount: 0,
				State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: created}},
			}},
		},
	}
}

func imagePullPod(name, namespace, uid, nodeName, ownerUID string, created metav1.Time) *corev1.Pod {
	controller := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			UID:               types.UID(uid),
			CreationTimestamp: created,
			Labels:            appLabels("checkout-api"),
			OwnerReferences:   []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "checkout-api-777b4dbdb8", UID: types.UID(ownerUID), Controller: &controller}},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name:  "api",
				Image: "registry.invalid.local/checkout-api:v1",
				Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}},
			}},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodPending,
			QOSClass: corev1.PodQOSBestEffort,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse, Reason: "ContainersNotReady"},
				{Type: corev1.ContainersReady, Status: corev1.ConditionFalse, Reason: "ContainersNotReady"},
			},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "api",
				Image:        "registry.invalid.local/checkout-api:v1",
				Ready:        false,
				RestartCount: 0,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason:  "ImagePullBackOff",
					Message: "Back-off pulling image \"registry.invalid.local/checkout-api:v1\"",
				}},
			}},
		},
	}
}

func crashLoopPod(name, namespace, uid, nodeName, ownerUID string, created metav1.Time) *corev1.Pod {
	controller := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			UID:               types.UID(uid),
			CreationTimestamp: created,
			Labels:            appLabels("payment-worker"),
			OwnerReferences:   []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "payment-worker-5c7d8f9d6d", UID: types.UID(ownerUID), Controller: &controller}},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name:    "worker",
				Image:   "busybox:1.36",
				Command: []string{"sh", "-c"},
				Args:    []string{"echo starting payment worker; exit 1"},
			}},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSBestEffort,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse, Reason: "ContainersNotReady"},
				{Type: corev1.ContainersReady, Status: corev1.ConditionFalse, Reason: "ContainersNotReady"},
			},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "worker",
				Image:        "busybox:1.36",
				Ready:        false,
				RestartCount: 7,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason:  "CrashLoopBackOff",
					Message: "back-off 5m0s restarting failed container=worker pod=payment-worker",
				}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode:   1,
					Reason:     "Error",
					Message:    "process exited with status 1",
					StartedAt:  metav1.NewTime(created.Time.Add(2 * time.Minute)),
					FinishedAt: metav1.NewTime(created.Time.Add(2*time.Minute + 4*time.Second)),
				}},
			}},
		},
	}
}

func probeFailurePod(name, namespace, uid, nodeName, podIP, ownerUID string, created metav1.Time) *corev1.Pod {
	controller := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			UID:               types.UID(uid),
			CreationTimestamp: created,
			Labels:            appLabels("profile-api"),
			OwnerReferences:   []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "profile-api-6b79cfd8b8", UID: types.UID(ownerUID), Controller: &controller}},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name:  "api",
				Image: "nginx:1.27",
				Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}},
				ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
					Path: "/fail",
					Port: intstr.FromInt32(8080),
				}}},
			}},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			PodIP:    podIP,
			QOSClass: corev1.PodQOSBurstable,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse, Reason: "ContainersNotReady"},
				{Type: corev1.ContainersReady, Status: corev1.ConditionFalse, Reason: "ContainersNotReady"},
			},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "api",
				Image:        "nginx:1.27",
				Ready:        false,
				RestartCount: 0,
				State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: created}},
			}},
		},
	}
}

func oomKilledPod(name, namespace, uid, nodeName, ownerUID string, created metav1.Time) *corev1.Pod {
	controller := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			UID:               types.UID(uid),
			CreationTimestamp: created,
			Labels:            appLabels("analytics-batch"),
			OwnerReferences:   []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "analytics-batch-764dbdd7cc", UID: types.UID(ownerUID), Controller: &controller}},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name:  "batch",
				Image: "busybox:1.36",
				Args:  []string{"simulate-oom"},
				Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("32Mi"),
				}},
			}},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			QOSClass: corev1.PodQOSBurstable,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse, Reason: "ContainersNotReady"},
				{Type: corev1.ContainersReady, Status: corev1.ConditionFalse, Reason: "ContainersNotReady"},
			},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "batch",
				Image:        "busybox:1.36",
				Ready:        false,
				RestartCount: 4,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason:  "CrashLoopBackOff",
					Message: "back-off restarting failed container=batch after OOMKilled",
				}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					ExitCode:   137,
					Reason:     "OOMKilled",
					Message:    "container was killed because it exceeded memory limit 32Mi",
					StartedAt:  metav1.NewTime(created.Time.Add(90 * time.Second)),
					FinishedAt: metav1.NewTime(created.Time.Add(120 * time.Second)),
				}},
			}},
		},
	}
}

func warningEvent(namespace, name, kind, reason, message string, count int32, eventTime metav1.Time) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:              strings.ToLower(kind) + "-" + name + "-" + strings.ToLower(reason),
			Namespace:         namespace,
			CreationTimestamp: eventTime,
		},
		InvolvedObject: corev1.ObjectReference{Kind: kind, Namespace: namespace, Name: name},
		Type:           corev1.EventTypeWarning,
		Reason:         reason,
		Message:        message,
		Count:          count,
		LastTimestamp:  eventTime,
	}
}

func pathTypePtr(value networkingv1.PathType) *networkingv1.PathType {
	return &value
}
