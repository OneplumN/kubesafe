package service

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SimulationScenarioResource struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Path      string `json:"path"`
}

type SimulationScenarioCommand struct {
	Title   string `json:"title"`
	Command string `json:"command"`
	Output  string `json:"output"`
}

type SimulationScenario struct {
	ID             string                       `json:"id"`
	Title          string                       `json:"title"`
	Category       string                       `json:"category"`
	Difficulty     string                       `json:"difficulty"`
	Namespace      string                       `json:"namespace"`
	Status         string                       `json:"status"`
	Completed      bool                         `json:"completed"`
	Symptom        string                       `json:"symptom"`
	Goal           string                       `json:"goal"`
	Diagnosis      string                       `json:"diagnosis"`
	FixHint        string                       `json:"fixHint"`
	EntryResources []SimulationScenarioResource `json:"entryResources"`
	Commands       []SimulationScenarioCommand  `json:"commands"`
}

type simulationScenarioDefinition struct {
	ID             string
	Title          string
	Category       string
	Difficulty     string
	Namespace      string
	Symptom        string
	Goal           string
	Diagnosis      string
	FixHint        string
	EntryResources []SimulationScenarioResource
	completed      func(context.Context, *ClusterService) (bool, string, error)
	reset          func(context.Context, *ClusterService) error
	commands       func(context.Context, *ClusterService) ([]SimulationScenarioCommand, error)
}

func (s *ClusterService) ListSimulationScenarios(ctx context.Context) ([]SimulationScenario, error) {
	if !s.simulationMode() {
		return nil, newValidationError("simulation scenarios are only available in simulation mode")
	}

	definitions := simulationScenarioDefinitions()
	items := make([]SimulationScenario, 0, len(definitions))
	for _, definition := range definitions {
		completed, status, err := definition.completed(ctx, s)
		if err != nil {
			return nil, err
		}
		commands, err := definition.commands(ctx, s)
		if err != nil {
			return nil, err
		}
		items = append(items, SimulationScenario{
			ID:             definition.ID,
			Title:          definition.Title,
			Category:       definition.Category,
			Difficulty:     definition.Difficulty,
			Namespace:      definition.Namespace,
			Status:         status,
			Completed:      completed,
			Symptom:        definition.Symptom,
			Goal:           definition.Goal,
			Diagnosis:      definition.Diagnosis,
			FixHint:        definition.FixHint,
			EntryResources: definition.EntryResources,
			Commands:       commands,
		})
	}

	return items, nil
}

func (s *ClusterService) ResetSimulationScenario(ctx context.Context, id string) (SimulationScenario, error) {
	if !s.simulationMode() {
		return SimulationScenario{}, newValidationError("simulation scenarios are only available in simulation mode")
	}

	id = strings.TrimSpace(id)
	for _, definition := range simulationScenarioDefinitions() {
		if definition.ID != id {
			continue
		}
		if err := definition.reset(ctx, s); err != nil {
			return SimulationScenario{}, err
		}
		completed, status, err := definition.completed(ctx, s)
		if err != nil {
			return SimulationScenario{}, err
		}
		commands, err := definition.commands(ctx, s)
		if err != nil {
			return SimulationScenario{}, err
		}
		return SimulationScenario{
			ID:             definition.ID,
			Title:          definition.Title,
			Category:       definition.Category,
			Difficulty:     definition.Difficulty,
			Namespace:      definition.Namespace,
			Status:         status,
			Completed:      completed,
			Symptom:        definition.Symptom,
			Goal:           definition.Goal,
			Diagnosis:      definition.Diagnosis,
			FixHint:        definition.FixHint,
			EntryResources: definition.EntryResources,
			Commands:       commands,
		}, nil
	}

	return SimulationScenario{}, newValidationError("simulation scenario %s not found", id)
}

func simulationScenarioDefinitions() []simulationScenarioDefinition {
	return []simulationScenarioDefinition{
		{
			ID:         "image-pull-backoff",
			Title:      "ImagePullBackOff: 镜像无法拉取",
			Category:   "Workload",
			Difficulty: "Beginner",
			Namespace:  "demo-broken",
			Symptom:    "checkout-api 的 Pod 卡在 ImagePullBackOff，Events 显示 registry.invalid.local 镜像拉取失败。",
			Goal:       "把 Deployment 镜像改为可拉取镜像，并让副本全部 Ready。",
			Diagnosis:  "检查 Pod Events、Container image 和 Deployment YAML。",
			FixHint:    "把镜像从 registry.invalid.local/checkout-api:v1 改成 nginx:1.27。",
			EntryResources: []SimulationScenarioResource{
				resourceLink("Deployment", "demo-broken", "checkout-api", "/workloads/deployments/demo-broken/checkout-api"),
				resourceLink("Pods", "demo-broken", "checkout-api", "/workloads/pods"),
			},
			completed: deploymentCompleted("demo-broken", "checkout-api", func(image string) bool {
				return !strings.Contains(image, "invalid")
			}),
			commands: workloadScenarioCommands("demo-broken", "checkout-api"),
			reset: resetDeploymentYAML("demo-broken", "checkout-api", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: checkout-api
  namespace: demo-broken
spec:
  replicas: 2
  selector:
    matchLabels:
      app: checkout-api
  template:
    metadata:
      labels:
        app: checkout-api
    spec:
      containers:
        - name: api
          image: registry.invalid.local/checkout-api:v1
          ports:
            - name: http
              containerPort: 8080
`),
		},
		{
			ID:         "crash-loop-backoff",
			Title:      "CrashLoopBackOff: 进程启动后退出",
			Category:   "Workload",
			Difficulty: "Beginner",
			Namespace:  "demo-broken",
			Symptom:    "payment-worker 持续重启，日志显示 process exited with status 1。",
			Goal:       "修复启动命令，让容器保持运行并 Ready。",
			Diagnosis:  "查看 Logs、Describe 的 Last State 和 Deployment command/args。",
			FixHint:    "把 args 里的 exit 1 改为 sleep 3600。",
			EntryResources: []SimulationScenarioResource{
				resourceLink("Deployment", "demo-broken", "payment-worker", "/workloads/deployments/demo-broken/payment-worker"),
				resourceLink("Pods", "demo-broken", "payment-worker", "/workloads/pods"),
			},
			completed: deploymentCompleted("demo-broken", "payment-worker", func(image string) bool { return image != "" }),
			commands:  workloadScenarioCommands("demo-broken", "payment-worker"),
			reset: resetDeploymentYAML("demo-broken", "payment-worker", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: payment-worker
  namespace: demo-broken
spec:
  replicas: 1
  selector:
    matchLabels:
      app: payment-worker
  template:
    metadata:
      labels:
        app: payment-worker
    spec:
      containers:
        - name: worker
          image: busybox:1.36
          command: ["sh", "-c"]
          args: ["echo starting payment worker; exit 1"]
`),
		},
		{
			ID:         "service-selector-mismatch",
			Title:      "Service selector 错配: Endpoints 为空",
			Category:   "Network",
			Difficulty: "Intermediate",
			Namespace:  "demo-workloads",
			Symptom:    "web-shadow Service 没有后端地址，Endpoints readyAddresses 为 0。",
			Goal:       "修正 Service selector，让它选中 web-portal 的 3 个 Pod。",
			Diagnosis:  "对比 Service selector 和目标 Pod labels。",
			FixHint:    "把 selector 从 app=web-portal-v2 改为 app=web-portal。",
			EntryResources: []SimulationScenarioResource{
				resourceLink("Service", "demo-workloads", "web-shadow", "/network/services/demo-workloads/web-shadow"),
				resourceLink("Endpoints", "demo-workloads", "web-shadow", "/network/endpoints/demo-workloads/web-shadow"),
			},
			completed: func(ctx context.Context, s *ClusterService) (bool, string, error) {
				if err := s.reconcileSimulationServiceEndpoints(ctx, "demo-workloads", "web-shadow"); err != nil {
					return false, "", err
				}
				endpoints, err := s.client.Kubernetes.CoreV1().Endpoints("demo-workloads").Get(ctx, "web-shadow", metav1.GetOptions{})
				if err != nil {
					return false, "", fmt.Errorf("get web-shadow endpoints: %w", err)
				}
				_, ready, _ := collectEndpointAddresses(*endpoints)
				return ready >= 3, fmt.Sprintf("%d ready endpoint(s)", ready), nil
			},
			commands: serviceScenarioCommands("demo-workloads", "web-shadow", "web-portal"),
			reset: resetServiceYAML("demo-workloads", "web-shadow", `apiVersion: v1
kind: Service
metadata:
  name: web-shadow
  namespace: demo-workloads
spec:
  type: ClusterIP
  clusterIP: 10.96.10.21
  selector:
    app: web-portal-v2
  ports:
    - name: http
      port: 80
      targetPort: 80
`),
		},
		{
			ID:         "readiness-probe-failed",
			Title:      "Readiness Probe 失败",
			Category:   "Workload",
			Difficulty: "Intermediate",
			Namespace:  "demo-broken",
			Symptom:    "profile-api Pod Running 但 Ready 0/1，日志显示 readiness probe HTTP 500。",
			Goal:       "修复 readinessProbe path，让 Deployment Ready。",
			Diagnosis:  "检查 Pod Conditions、Events 和 Deployment readinessProbe。",
			FixHint:    "把 readinessProbe.httpGet.path 从 /fail 改为 /healthz。",
			EntryResources: []SimulationScenarioResource{
				resourceLink("Deployment", "demo-broken", "profile-api", "/workloads/deployments/demo-broken/profile-api"),
			},
			completed: deploymentCompleted("demo-broken", "profile-api", func(image string) bool { return image != "" }),
			commands:  workloadScenarioCommands("demo-broken", "profile-api"),
			reset: resetDeploymentYAML("demo-broken", "profile-api", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-api
  namespace: demo-broken
spec:
  replicas: 1
  selector:
    matchLabels:
      app: profile-api
  template:
    metadata:
      labels:
        app: profile-api
    spec:
      containers:
        - name: api
          image: nginx:1.27
          ports:
            - name: http
              containerPort: 8080
          readinessProbe:
            httpGet:
              path: /fail
              port: 8080
`),
		},
		{
			ID:         "oom-killed",
			Title:      "OOMKilled: 内存限制过低",
			Category:   "Workload",
			Difficulty: "Intermediate",
			Namespace:  "demo-broken",
			Symptom:    "analytics-batch Last State 为 OOMKilled，Exit Code 137。",
			Goal:       "提高内存限制并去掉触发 OOM 的参数，让 Pod Ready。",
			Diagnosis:  "查看 Logs、Describe Last State 和 container resources.limits.memory。",
			FixHint:    "把 memory limit 改为 256Mi，并把 args 改成 sleep 3600。",
			EntryResources: []SimulationScenarioResource{
				resourceLink("Deployment", "demo-broken", "analytics-batch", "/workloads/deployments/demo-broken/analytics-batch"),
			},
			completed: deploymentCompleted("demo-broken", "analytics-batch", func(image string) bool { return image != "" }),
			commands:  workloadScenarioCommands("demo-broken", "analytics-batch"),
			reset: resetDeploymentYAML("demo-broken", "analytics-batch", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: analytics-batch
  namespace: demo-broken
spec:
  replicas: 1
  selector:
    matchLabels:
      app: analytics-batch
  template:
    metadata:
      labels:
        app: analytics-batch
    spec:
      containers:
        - name: batch
          image: busybox:1.36
          args: ["simulate-oom"]
          resources:
            limits:
              memory: 32Mi
`),
		},
		{
			ID:         "pvc-pending",
			Title:      "PVC Pending: StorageClass 不存在",
			Category:   "Storage",
			Difficulty: "Beginner",
			Namespace:  "demo-broken",
			Symptom:    "report-cache PVC 使用 fast-ssd，事件显示 StorageClass not found。",
			Goal:       "把 PVC 改到存在的 local-path StorageClass 并绑定。",
			Diagnosis:  "检查 PVC status、Events 和 StorageClass 列表。",
			FixHint:    "把 storageClassName 从 fast-ssd 改为 local-path。",
			EntryResources: []SimulationScenarioResource{
				resourceLink("PVC", "demo-broken", "report-cache", "/storage/persistentvolumeclaims/demo-broken/report-cache"),
				resourceLink("StorageClass", "", "local-path", "/storage/storageclasses/local-path"),
			},
			completed: func(ctx context.Context, s *ClusterService) (bool, string, error) {
				claim, err := s.client.Kubernetes.CoreV1().PersistentVolumeClaims("demo-broken").Get(ctx, "report-cache", metav1.GetOptions{})
				if err != nil {
					return false, "", fmt.Errorf("get report-cache pvc: %w", err)
				}
				return claim.Status.Phase == corev1.ClaimBound, string(claim.Status.Phase), nil
			},
			commands: pvcScenarioCommands("demo-broken", "report-cache"),
			reset: resetPVCYAML("demo-broken", "report-cache", `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: report-cache
  namespace: demo-broken
spec:
  storageClassName: fast-ssd
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
`),
		},
	}
}

func resourceLink(kind string, namespace string, name string, path string) SimulationScenarioResource {
	return SimulationScenarioResource{Kind: kind, Namespace: namespace, Name: name, Path: path}
}

func workloadScenarioCommands(namespace string, deploymentName string) func(context.Context, *ClusterService) ([]SimulationScenarioCommand, error) {
	return func(ctx context.Context, s *ClusterService) ([]SimulationScenarioCommand, error) {
		deployments, err := s.ListDeployments(ctx, namespace)
		if err != nil {
			return nil, err
		}
		pods, err := s.ListPods(ctx, namespace)
		if err != nil {
			return nil, err
		}

		var deployment *DeploymentItem
		for i := range deployments {
			if deployments[i].Name == deploymentName {
				deployment = &deployments[i]
				break
			}
		}

		matchedPods := podsForApp(pods, deploymentName)
		var firstPod *PodItem
		if len(matchedPods) > 0 {
			firstPod = &matchedPods[0]
		}

		commands := []SimulationScenarioCommand{
			{
				Title:   "Deployment 摘要",
				Command: fmt.Sprintf("kubectl get deploy -n %s %s", namespace, deploymentName),
				Output:  formatKubectlDeployment(deployment),
			},
			{
				Title:   "关联 Pod 列表",
				Command: fmt.Sprintf("kubectl get pods -n %s -l app=%s -o wide", namespace, deploymentName),
				Output:  formatKubectlPods(matchedPods),
			},
		}

		if firstPod == nil {
			commands = append(commands, SimulationScenarioCommand{
				Title:   "Pod Describe",
				Command: fmt.Sprintf("kubectl describe pod -n %s <pod>", namespace),
				Output:  "No resources found.",
			})
			return commands, nil
		}

		describe, err := s.GetPodDescribe(ctx, namespace, firstPod.Name)
		if err != nil {
			return nil, err
		}
		container := ""
		if len(firstPod.Containers) > 0 {
			container = firstPod.Containers[0].Name
		}
		logs, err := s.GetPodLogs(ctx, namespace, firstPod.Name, container, 20)
		if err != nil {
			return nil, err
		}

		commands = append(commands,
			SimulationScenarioCommand{
				Title:   "Pod Describe",
				Command: fmt.Sprintf("kubectl describe pod -n %s %s", namespace, firstPod.Name),
				Output:  describe.Content,
			},
			SimulationScenarioCommand{
				Title:   "容器日志",
				Command: fmt.Sprintf("kubectl logs -n %s %s -c %s --tail=20", namespace, firstPod.Name, container),
				Output:  logs.Content,
			},
		)

		return commands, nil
	}
}

func serviceScenarioCommands(namespace string, serviceName string, targetApp string) func(context.Context, *ClusterService) ([]SimulationScenarioCommand, error) {
	return func(ctx context.Context, s *ClusterService) ([]SimulationScenarioCommand, error) {
		services, err := s.ListServices(ctx, namespace)
		if err != nil {
			return nil, err
		}
		endpoints, err := s.ListEndpoints(ctx, namespace)
		if err != nil {
			return nil, err
		}
		pods, err := s.ListPods(ctx, namespace)
		if err != nil {
			return nil, err
		}

		var service *ServiceItem
		for i := range services {
			if services[i].Name == serviceName {
				service = &services[i]
				break
			}
		}

		var endpoint *EndpointItem
		for i := range endpoints {
			if endpoints[i].Name == serviceName {
				endpoint = &endpoints[i]
				break
			}
		}

		return []SimulationScenarioCommand{
			{
				Title:   "Service 摘要",
				Command: fmt.Sprintf("kubectl get svc -n %s %s -o wide", namespace, serviceName),
				Output:  formatKubectlService(service),
			},
			{
				Title:   "Endpoints",
				Command: fmt.Sprintf("kubectl get endpoints -n %s %s", namespace, serviceName),
				Output:  formatKubectlEndpoints(endpoint),
			},
			{
				Title:   "目标 Pod Labels",
				Command: fmt.Sprintf("kubectl get pods -n %s -l app=%s --show-labels", namespace, targetApp),
				Output:  formatKubectlPodsWithLabels(podsForApp(pods, targetApp)),
			},
		}, nil
	}
}

func pvcScenarioCommands(namespace string, claimName string) func(context.Context, *ClusterService) ([]SimulationScenarioCommand, error) {
	return func(ctx context.Context, s *ClusterService) ([]SimulationScenarioCommand, error) {
		claims, err := s.ListPersistentVolumeClaims(ctx, namespace)
		if err != nil {
			return nil, err
		}

		var claim *PersistentVolumeClaimItem
		for i := range claims {
			if claims[i].Name == claimName {
				claim = &claims[i]
				break
			}
		}

		return []SimulationScenarioCommand{
			{
				Title:   "PVC 摘要",
				Command: fmt.Sprintf("kubectl get pvc -n %s %s", namespace, claimName),
				Output:  formatKubectlPVC(claim),
			},
			{
				Title:   "PVC Describe",
				Command: fmt.Sprintf("kubectl describe pvc -n %s %s", namespace, claimName),
				Output:  formatKubectlPVCDescribe(claim),
			},
			{
				Title:   "StorageClass 列表",
				Command: "kubectl get storageclass",
				Output:  "NAME                   PROVISIONER                    RECLAIMPOLICY   VOLUMEBINDINGMODE\nlocal-path (default)   rancher.io/local-path          Delete          WaitForFirstConsumer\nstandard               kubernetes.io/no-provisioner   Delete          Immediate",
			},
		}, nil
	}
}

func podsForApp(pods []PodItem, app string) []PodItem {
	result := make([]PodItem, 0)
	for _, pod := range pods {
		if hasLabelPair(pod.Labels, "app="+app) {
			result = append(result, pod)
		}
	}
	return result
}

func hasLabelPair(labels []string, expected string) bool {
	for _, label := range labels {
		if label == expected {
			return true
		}
	}
	return false
}

func formatKubectlDeployment(item *DeploymentItem) string {
	if item == nil {
		return "No resources found."
	}
	return fmt.Sprintf(
		"NAME             READY   UP-TO-DATE   AVAILABLE   AGE   CONTAINERS   IMAGES\n%s   %d/%d     %d            %d           %s   %s   %s",
		item.Name,
		item.ReadyReplicas,
		item.DesiredReplicas,
		item.UpdatedReplicas,
		item.AvailableReplicas,
		item.Age,
		strings.Join(containerNamesFromImages(item.Images), ","),
		strings.Join(imageRefs(item.Images), ","),
	)
}

func formatKubectlPods(items []PodItem) string {
	if len(items) == 0 {
		return "No resources found."
	}
	var builder strings.Builder
	builder.WriteString("NAME                                READY   STATUS             RESTARTS   AGE   IP           NODE\n")
	for _, item := range items {
		builder.WriteString(fmt.Sprintf(
			"%-35s %d/%d     %-18s %-10d %-5s %-12s %s\n",
			item.Name,
			item.ReadyContainers,
			item.TotalContainers,
			item.Status,
			item.RestartCount,
			item.Age,
			defaultString(item.PodIP, "<none>"),
			defaultString(item.NodeName, "<none>"),
		))
	}
	return strings.TrimRight(builder.String(), "\n")
}

func formatKubectlPodsWithLabels(items []PodItem) string {
	if len(items) == 0 {
		return "No resources found."
	}
	var builder strings.Builder
	builder.WriteString("NAME                                READY   STATUS    RESTARTS   AGE   LABELS\n")
	for _, item := range items {
		builder.WriteString(fmt.Sprintf(
			"%-35s %d/%d     %-9s %-10d %-5s %s\n",
			item.Name,
			item.ReadyContainers,
			item.TotalContainers,
			item.Status,
			item.RestartCount,
			item.Age,
			strings.Join(item.Labels, ","),
		))
	}
	return strings.TrimRight(builder.String(), "\n")
}

func formatKubectlService(item *ServiceItem) string {
	if item == nil {
		return "No resources found."
	}
	return fmt.Sprintf(
		"NAME         TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)   AGE   SELECTOR\n%s   %-11s %-13s <none>        %-9s %s   %s",
		item.Name,
		item.Type,
		item.ClusterIP,
		item.PortsSummary,
		item.Age,
		strings.Join(item.Selector, ","),
	)
}

func formatKubectlEndpoints(item *EndpointItem) string {
	if item == nil {
		return "No resources found."
	}
	addresses := make([]string, 0, len(item.Addresses))
	for _, address := range item.Addresses {
		if address.Ready {
			addresses = append(addresses, address.IP)
		}
	}
	if len(addresses) == 0 {
		addresses = append(addresses, "<none>")
	}
	return fmt.Sprintf(
		"NAME         ENDPOINTS          AGE\n%s   %-18s %s",
		item.Name,
		strings.Join(addresses, ","),
		item.Age,
	)
}

func formatKubectlPVC(item *PersistentVolumeClaimItem) string {
	if item == nil {
		return "No resources found."
	}
	return fmt.Sprintf(
		"NAME           STATUS   VOLUME       CAPACITY   ACCESS MODES   STORAGECLASS   AGE\n%s   %-8s %-12s %-10s %-14s %-14s %s",
		item.Name,
		item.Status,
		defaultString(item.VolumeName, "<none>"),
		defaultString(item.Capacity, item.RequestedStorage),
		strings.Join(item.AccessModes, ","),
		item.StorageClass,
		item.Age,
	)
}

func formatKubectlPVCDescribe(item *PersistentVolumeClaimItem) string {
	if item == nil {
		return "No resources found."
	}

	eventMessage := "waiting for a volume to be created, either by external provisioner or manually created by system administrator"
	if item.StorageClass == "fast-ssd" {
		eventMessage = `storageclass.storage.k8s.io "fast-ssd" not found`
	}
	if item.Status == TopologyStatusHealthy {
		eventMessage = "Successfully provisioned volume pvc-" + item.Name
	}

	return fmt.Sprintf(`Name:          %s
Namespace:     %s
StorageClass:  %s
Status:        %s
Volume:        %s
Labels:        %s
Capacity:      %s
Access Modes:  %s
Mounted By:    %s
Events:
  Type     Reason                Age   From                         Message
  ----     ------                ----  ----                         -------
  Warning  ProvisioningFailed    1m    persistentvolume-controller  %s`,
		item.Name,
		item.Namespace,
		item.StorageClass,
		item.Status,
		defaultString(item.VolumeName, "<none>"),
		defaultString(strings.Join(item.Labels, ","), "<none>"),
		defaultString(item.Capacity, item.RequestedStorage),
		strings.Join(item.AccessModes, ","),
		defaultString(strings.Join(item.MountedPods, ","), "<none>"),
		eventMessage,
	)
}

func containerNamesFromImages(images []string) []string {
	if len(images) == 0 {
		return []string{"<none>"}
	}
	names := make([]string, 0, len(images))
	for _, image := range images {
		container, _ := splitContainerImage(image)
		names = append(names, container)
	}
	return names
}

func imageRefs(images []string) []string {
	if len(images) == 0 {
		return []string{"<none>"}
	}
	refs := make([]string, 0, len(images))
	for _, image := range images {
		_, ref := splitContainerImage(image)
		refs = append(refs, ref)
	}
	return refs
}

func splitContainerImage(value string) (string, string) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) == 2 {
		return defaultString(parts[0], "app"), defaultString(parts[1], "<none>")
	}
	return "app", defaultString(value, "<none>")
}

func deploymentCompleted(namespace string, name string, imageCheck func(string) bool) func(context.Context, *ClusterService) (bool, string, error) {
	return func(ctx context.Context, s *ClusterService) (bool, string, error) {
		items, err := s.ListDeployments(ctx, namespace)
		if err != nil {
			return false, "", err
		}
		for _, item := range items {
			if item.Name != name {
				continue
			}
			imageOK := true
			if len(item.Images) > 0 {
				imageOK = imageCheck(strings.Join(item.Images, ","))
			}
			completed := item.Status == TopologyStatusHealthy && item.UnavailableReplicas == 0 && item.ReadyReplicas == item.DesiredReplicas && imageOK
			return completed, fmt.Sprintf("%s · ready %d/%d", item.Status, item.ReadyReplicas, item.DesiredReplicas), nil
		}
		return false, "missing", nil
	}
}

func resetDeploymentYAML(namespace string, name string, content string) func(context.Context, *ClusterService) error {
	return func(ctx context.Context, s *ClusterService) error {
		_, err := s.applySimulationResourceYAML(ctx, "Deployment", namespace, name, content)
		return err
	}
}

func resetServiceYAML(namespace string, name string, content string) func(context.Context, *ClusterService) error {
	return func(ctx context.Context, s *ClusterService) error {
		_, err := s.applySimulationResourceYAML(ctx, "Service", namespace, name, content)
		return err
	}
}

func resetPVCYAML(namespace string, name string, content string) func(context.Context, *ClusterService) error {
	return func(ctx context.Context, s *ClusterService) error {
		_, err := s.applySimulationResourceYAML(ctx, "PersistentVolumeClaim", namespace, name, content)
		return err
	}
}
