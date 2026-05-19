import {
  BulbOutlined,
  CodeOutlined,
  DeploymentUnitOutlined,
  FileTextOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  RightOutlined,
} from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  App,
  Button,
  Col,
  Input,
  Popconfirm,
  Row,
  Select,
  Space,
  Tag,
  Typography,
} from 'antd';
import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import {
  applyManifests,
  getDeployments,
  getPersistentVolumeClaims,
  getPodDescribe,
  getPodEvents,
  getPodLogs,
  getPods,
  getServices,
  resetSimulationNamespace,
  type DeploymentItem,
  type ManifestBatchResult,
  type PersistentVolumeClaimItem,
  type PodEventItem,
  type PodItem,
  type ServiceItem,
  type WorkloadActionResult,
} from '../services/cluster';

type ManifestIdentity = {
  apiVersion: string;
  kind: string;
  namespace: string;
  name: string;
};

type TemplateItem = {
  key: string;
  label: string;
  description: string;
  content: string;
};

type DiagnosisItem = {
  level: 'error' | 'warning' | 'success' | 'info';
  title: string;
  detail: string;
  field: string;
  fix: string;
  action?: 'fix-image' | 'fix-configmap' | 'fix-requests' | 'fix-crash' | 'fix-readiness' | 'fix-service-selector' | 'fix-pvc';
  actionLabel?: string;
  payload?: string;
};

const templates: TemplateItem[] = [
  {
    key: 'healthy-deployment',
    label: '健康 Deployment',
    description: '创建一个可以 Ready 的 nginx Deployment。',
    content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: yaml-nginx
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: yaml-nginx
  template:
    metadata:
      labels:
        app: yaml-nginx
    spec:
      containers:
        - name: nginx
          image: nginx:1.27
          ports:
            - containerPort: 80
`,
  },
  {
    key: 'image-pull',
    label: '镜像拉取失败',
    description: '使用不存在的镜像，触发 ImagePullBackOff。',
    content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: yaml-image-pull
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: yaml-image-pull
  template:
    metadata:
      labels:
        app: yaml-image-pull
    spec:
      containers:
        - name: app
          image: registry.invalid.local/demo-api:missing
`,
  },
  {
    key: 'missing-configmap',
    label: '缺少 ConfigMap',
    description: '引用不存在的 ConfigMap，触发 CreateContainerConfigError。',
    content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: yaml-config-error
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: yaml-config-error
  template:
    metadata:
      labels:
        app: yaml-config-error
    spec:
      containers:
        - name: app
          image: nginx:1.27
          envFrom:
            - configMapRef:
                name: missing-app-config
`,
  },
  {
    key: 'unschedulable',
    label: '资源请求过大',
    description: '请求超过模拟集群容量，触发 Unschedulable。',
    content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: yaml-too-large
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: yaml-too-large
  template:
    metadata:
      labels:
        app: yaml-too-large
    spec:
      containers:
        - name: app
          image: nginx:1.27
          resources:
            requests:
              cpu: "20"
              memory: 40Gi
`,
  },
  {
    key: 'bad-readiness',
    label: 'Readiness 失败',
    description: '探针路径异常，Pod Running 但 NotReady。',
    content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: yaml-bad-readiness
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: yaml-bad-readiness
  template:
    metadata:
      labels:
        app: yaml-bad-readiness
    spec:
      containers:
        - name: app
          image: nginx:1.27
          ports:
            - containerPort: 8080
          readinessProbe:
            httpGet:
              path: /fail
              port: 8080
`,
  },
  {
    key: 'service-selector',
    label: 'Service selector 错配',
    description: 'Service selector 指向不存在的 label，Endpoints 为空。',
    content: `apiVersion: v1
kind: Service
metadata:
  name: yaml-shadow-service
  namespace: default
spec:
  type: ClusterIP
  selector:
    app: yaml-nginx-v2
  ports:
    - name: http
      port: 80
      targetPort: 80
`,
  },
  {
    key: 'deployment-service',
    label: 'Deployment + Service',
    description: '一次提交两个资源，观察 Service 自动生成 Endpoints。',
    content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: yaml-web-stack
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: yaml-web-stack
  template:
    metadata:
      labels:
        app: yaml-web-stack
    spec:
      containers:
        - name: web
          image: nginx:1.27
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: yaml-web-stack
  namespace: default
spec:
  type: ClusterIP
  selector:
    app: yaml-web-stack
  ports:
    - name: http
      port: 80
      targetPort: 80
`,
  },
  {
    key: 'configmap-before-deploy',
    label: 'ConfigMap + Deployment',
    description: '一次提交配置和引用它的工作负载，观察配置错误消失。',
    content: `apiVersion: v1
kind: ConfigMap
metadata:
  name: yaml-app-config
  namespace: default
data:
  APP_MODE: simulation
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: yaml-app-config
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: yaml-app-config
  template:
    metadata:
      labels:
        app: yaml-app-config
    spec:
      containers:
        - name: app
          image: nginx:1.27
          envFrom:
            - configMapRef:
                name: yaml-app-config
`,
  },
  {
    key: 'pvc-pending',
    label: 'PVC Pending',
    description: '使用不存在的 StorageClass，触发 PVC Pending。',
    content: `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: yaml-cache
  namespace: default
spec:
  storageClassName: fast-ssd
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
`,
  },
];

const defaultTemplate = templates[0];

function instantiateTemplate(content: string) {
  const identity = parseManifestIdentity(content);
  if (!identity.name) {
    return content;
  }

  const nextName = `${identity.name}-${Date.now().toString(36).slice(-5)}`;
  return content.replaceAll(identity.name, nextName);
}

function primaryResult(batch?: ManifestBatchResult) {
  if (!batch) {
    return undefined;
  }
  return (
    batch.results.find((item) => item.kind === 'Deployment') ??
    batch.results.find((item) => item.kind === 'Service') ??
    batch.results.find((item) => item.kind === 'PersistentVolumeClaim') ??
    batch.results[0]
  );
}

function parseManifestIdentity(content: string): ManifestIdentity {
  const readTopLevel = (key: string) => {
    const match = content.match(new RegExp(`^${key}:\\s*([^\\n#]+)`, 'm'));
    return match?.[1]?.trim().replace(/^['"]|['"]$/g, '') ?? '';
  };

  const metadataBlock = content.match(/^metadata:\s*\n([\s\S]*?)(?=^[^ \n]|$)/m)?.[1] ?? '';
  const readMetadata = (key: string) => {
    const match = metadataBlock.match(new RegExp(`^\\s{2}${key}:\\s*([^\\n#]+)`, 'm'));
    return match?.[1]?.trim().replace(/^['"]|['"]$/g, '') ?? '';
  };

  return {
    apiVersion: readTopLevel('apiVersion'),
    kind: readTopLevel('kind'),
    namespace: readMetadata('namespace') || 'default',
    name: readMetadata('name'),
  };
}

function statusColor(status: string) {
  switch (status) {
    case 'Running':
    case 'Healthy':
    case 'Bound':
      return 'green';
    case 'Pending':
    case 'Warning':
      return 'orange';
    case 'ImagePullBackOff':
    case 'CrashLoopBackOff':
    case 'CreateContainerConfigError':
    case 'Unschedulable':
    case 'Error':
    case 'Degraded':
      return 'red';
    default:
      return 'blue';
  }
}

function resourcePath(result: WorkloadActionResult) {
  const namespace = encodeURIComponent(result.namespace || 'default');
  const name = encodeURIComponent(result.name);
  switch (result.kind) {
    case 'Deployment':
      return `/workloads/deployments/${namespace}/${name}`;
    case 'Service':
      return `/network/services/${namespace}/${name}`;
    case 'PersistentVolumeClaim':
      return `/storage/persistentvolumeclaims/${namespace}/${name}`;
    case 'ConfigMap':
      return `/config/configmaps/${namespace}/${name}`;
    case 'Secret':
      return `/config/secrets/${namespace}/${name}`;
    case 'Ingress':
      return `/network/ingresses/${namespace}/${name}`;
    default:
      return '';
  }
}

function podsForResult(pods: PodItem[], result?: WorkloadActionResult) {
  if (!result) {
    return [];
  }
  return pods.filter(
    (pod) =>
      pod.namespace === result.namespace &&
      (pod.name.includes(result.name) || pod.labels.includes(`app=${result.name}`)),
  );
}

function buildDiagnosis(params: {
  content: string;
  pods: PodItem[];
  deployment?: DeploymentItem;
  service?: ServiceItem;
  pvc?: PersistentVolumeClaimItem;
}): DiagnosisItem[] {
  const { content, pods, deployment, service, pvc } = params;
  const diagnoses: DiagnosisItem[] = [];
  const lowerContent = content.toLowerCase();
  const unhealthyPods = pods.filter((pod) => pod.status !== 'Running' || pod.readyContainers < pod.totalContainers);

  for (const pod of unhealthyPods) {
    const container = pod.containers[0];
    const stateMessage = container?.stateMessage ?? '';
    if (pod.status === 'ImagePullBackOff' || pod.status === 'ErrImagePull') {
      diagnoses.push({
        level: 'error',
        title: '镜像拉取失败',
        detail: `${pod.name} 无法拉取镜像 ${container?.image ?? '-'}。`,
        field: 'spec.template.spec.containers[].image',
        fix: '改成可拉取镜像，例如 nginx:1.27，或补齐私有仓库凭证 imagePullSecrets。',
        action: 'fix-image',
        actionLabel: '替换为 nginx:1.27',
        payload: container?.image,
      });
      continue;
    }
    if (pod.status === 'CreateContainerConfigError') {
      diagnoses.push({
        level: 'error',
        title: '容器配置引用缺失',
        detail: stateMessage || `${pod.name} 在创建容器配置时失败。`,
        field: 'spec.template.spec.containers[].envFrom / env[].valueFrom',
        fix: '创建对应 ConfigMap/Secret，或把引用名改成已经存在的配置资源。',
        action: 'fix-configmap',
        actionLabel: '补一个 ConfigMap',
        payload: missingConfigName(stateMessage),
      });
      continue;
    }
    if (pod.status === 'Unschedulable') {
      const condition = pod.conditions.find((item) => item.type === 'PodScheduled');
      diagnoses.push({
        level: 'error',
        title: '调度失败',
        detail: condition?.message || `${pod.name} 无法被调度到当前模拟节点。`,
        field: 'spec.template.spec.containers[].resources.requests',
        fix: '降低 cpu/memory requests，或扩展模拟集群容量。',
        action: 'fix-requests',
        actionLabel: '降低资源请求',
      });
      continue;
    }
    if (pod.status === 'CrashLoopBackOff') {
      diagnoses.push({
        level: 'error',
        title: '进程反复退出',
        detail: stateMessage || `${pod.name} 的容器启动后退出并进入退避重启。`,
        field: 'spec.template.spec.containers[].command / args',
        fix: '检查启动命令和参数，避免 exit 1、panic、throw 等立即退出行为。',
        action: 'fix-crash',
        actionLabel: '改成 sleep 3600',
      });
      continue;
    }
    if (pod.readyContainers < pod.totalContainers && pod.phase === 'Running') {
      diagnoses.push({
        level: 'warning',
        title: 'Pod Running 但未 Ready',
        detail: `${pod.name} 已运行，但 Ready ${pod.readyContainers}/${pod.totalContainers}。`,
        field: 'spec.template.spec.containers[].readinessProbe',
        fix: '检查 readinessProbe path/port，避免指向 /fail、/bad、/404 这类失败路径。',
        action: 'fix-readiness',
        actionLabel: '改探针为 /healthz',
      });
    }
  }

  if (deployment && deployment.status === 'Healthy' && pods.length > 0) {
    diagnoses.push({
      level: 'success',
      title: '工作负载健康',
      detail: `${deployment.namespace}/${deployment.name} 已达到 Ready ${deployment.readyReplicas}/${deployment.desiredReplicas}。`,
      field: 'Deployment status',
      fix: '可以继续添加 Service、Ingress、PVC 或故障参数观察联动结果。',
    });
  }

  if (service) {
    if (service.podCount === 0) {
      diagnoses.push({
        level: 'warning',
        title: 'Service 没有匹配后端 Pod',
        detail: `${service.namespace}/${service.name} selector=${service.selector.join(', ') || '-'} 没有选中 Pod。`,
        field: 'spec.selector',
        fix: '让 Service selector 与目标 Pod labels 对齐，例如 app 与 Deployment template labels 保持一致。',
        action: 'fix-service-selector',
        actionLabel: '对齐 app selector',
        payload: firstAppLabel(pods, content),
      });
    } else {
      diagnoses.push({
        level: 'success',
        title: 'Service 已匹配后端',
        detail: `${service.namespace}/${service.name} 当前匹配 ${service.podCount} 个 Pod。`,
        field: 'spec.selector',
        fix: '可进入 Endpoints 页面查看 readyAddresses 是否符合预期。',
      });
    }
  } else if (lowerContent.includes('kind: service')) {
    diagnoses.push({
      level: 'info',
      title: 'Service 状态仍在刷新',
      detail: '本次 YAML 包含 Service，但列表查询暂未返回对应服务。',
      field: 'kind: Service',
      fix: '点击刷新模拟状态，或检查 Service metadata.name/namespace。',
    });
  }

  if (pvc) {
    if (pvc.status === 'Pending') {
      diagnoses.push({
        level: 'warning',
        title: 'PVC 未绑定',
        detail: `${pvc.namespace}/${pvc.name} 使用 StorageClass ${pvc.storageClass}，当前仍为 Pending。`,
        field: 'spec.storageClassName',
        fix: '改成模拟集群存在的 local-path，或创建对应 StorageClass。',
        action: 'fix-pvc',
        actionLabel: '改成 local-path',
      });
    } else {
      diagnoses.push({
        level: 'success',
        title: 'PVC 已绑定',
        detail: `${pvc.namespace}/${pvc.name} 已绑定到 ${pvc.volumeName ?? '模拟 PV'}。`,
        field: 'status.phase',
        fix: '可以继续创建挂载该 PVC 的 Pod/Deployment 验证存储链路。',
      });
    }
  }

  if (diagnoses.length === 0 && lastInterestingKind(content)) {
    diagnoses.push({
      level: 'info',
      title: '暂无异常信号',
      detail: '当前资源没有触发已知异常规则。',
      field: lastInterestingKind(content),
      fix: '可以尝试改 image、envFrom、resources.requests、readinessProbe、service.selector 或 storageClassName。',
    });
  }

  return diagnoses;
}

function missingConfigName(message: string) {
  return /configmap "([^"]+)"/i.exec(message)?.[1] ?? /secret "([^"]+)"/i.exec(message)?.[1] ?? '';
}

function firstAppLabel(pods: PodItem[], content: string) {
  for (const pod of pods) {
    const label = pod.labels.find((item) => item.startsWith('app='));
    if (label) {
      return label.slice('app='.length);
    }
  }
  const labels = [...content.matchAll(/app:\s*([^\n#]+)/g)].map((match) => match[1]?.trim()).filter(Boolean);
  return labels[0] ?? '';
}

function applyQuickFix(content: string, diagnosis: DiagnosisItem) {
  switch (diagnosis.action) {
    case 'fix-image':
      if (diagnosis.payload) {
        return content.replace(diagnosis.payload, 'nginx:1.27');
      }
      return content.replace(/^(\s*image:\s*).+$/m, '$1nginx:1.27');
    case 'fix-configmap':
      return prependConfigMap(content, diagnosis.payload || 'app-config');
    case 'fix-requests':
      return content
        .replace(/^(\s*cpu:\s*).+$/m, '$1"250m"')
        .replace(/^(\s*memory:\s*).+$/m, (_match, prefix: string) => `${prefix}256Mi`);
    case 'fix-crash':
      return content
        .replace(/^(\s*args:\s*)\[.*exit 1.*\]$/m, '$1["sleep 3600"]')
        .replace(/exit 1/g, 'sleep 3600')
        .replace(/panic/g, 'sleep 3600')
        .replace(/throw/g, 'sleep 3600');
    case 'fix-readiness':
      return content.replace(/path:\s*\/(?:fail|bad|404)\b/g, 'path: /healthz');
    case 'fix-service-selector':
      if (!diagnosis.payload) {
        return content;
      }
      return content.replace(
        /(\n\s*selector:\s*\n\s*app:\s*)[^\n#]+/m,
        (_match, prefix: string) => `${prefix}${diagnosis.payload}`,
      );
    case 'fix-pvc':
      return content.replace(/^(\s*storageClassName:\s*).+$/m, '$1local-path');
    default:
      return content;
  }
}

function prependConfigMap(content: string, name: string) {
  const identity = parseManifestIdentity(content);
  const namespace = identity.namespace || 'default';
  const configMap = `apiVersion: v1
kind: ConfigMap
metadata:
  name: ${name}
  namespace: ${namespace}
data:
  APP_MODE: simulation
`;
  if (content.includes(`kind: ConfigMap`) && content.includes(`name: ${name}`)) {
    return content;
  }
  return `${configMap}---\n${content}`;
}

function lastInterestingKind(content: string) {
  const matches = [...content.matchAll(/^kind:\s*([^\n#]+)/gm)].map((match) => match[1]?.trim()).filter(Boolean);
  return matches.at(-1) ?? '';
}

export function SimulationYamlLabPage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [content, setContent] = useState(() => instantiateTemplate(defaultTemplate.content));
  const [lastBatch, setLastBatch] = useState<ManifestBatchResult>();

  const identity = useMemo(() => parseManifestIdentity(content), [content]);
  const lastResult = primaryResult(lastBatch);
  const namespace = lastResult?.namespace || identity.namespace || 'default';

  const podsQuery = useQuery({
    queryKey: ['yaml-lab-pods', namespace, lastResult?.name],
    queryFn: () => getPods(namespace),
    enabled: Boolean(lastResult),
  });

  const deploymentsQuery = useQuery({
    queryKey: ['yaml-lab-deployments', namespace, lastResult?.name],
    queryFn: () => getDeployments(namespace),
    enabled: Boolean(lastBatch?.results.some((item) => item.kind === 'Deployment')),
  });

  const servicesQuery = useQuery({
    queryKey: ['yaml-lab-services', namespace, lastResult?.name],
    queryFn: () => getServices(namespace),
    enabled: Boolean(lastBatch?.results.some((item) => item.kind === 'Service')),
  });

  const pvcQuery = useQuery({
    queryKey: ['yaml-lab-pvcs', namespace, lastResult?.name],
    queryFn: () => getPersistentVolumeClaims(namespace),
    enabled: Boolean(lastBatch?.results.some((item) => item.kind === 'PersistentVolumeClaim')),
  });

  const applyMutation = useMutation({
    mutationFn: applyManifests,
    onSuccess: async (result) => {
      setLastBatch(result);
      void message.success(result.message);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['pods'] }),
        queryClient.invalidateQueries({ queryKey: ['deployments'] }),
        queryClient.invalidateQueries({ queryKey: ['services'] }),
        queryClient.invalidateQueries({ queryKey: ['endpoints'] }),
        queryClient.invalidateQueries({ queryKey: ['persistentvolumeclaims'] }),
      ]);
    },
  });

  const resetMutation = useMutation({
    mutationFn: resetSimulationNamespace,
    onSuccess: async (result) => {
      setLastBatch(undefined);
      void message.success(result.message);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['pods'] }),
        queryClient.invalidateQueries({ queryKey: ['deployments'] }),
        queryClient.invalidateQueries({ queryKey: ['services'] }),
        queryClient.invalidateQueries({ queryKey: ['endpoints'] }),
        queryClient.invalidateQueries({ queryKey: ['persistentvolumeclaims'] }),
        podsQuery.refetch(),
        deploymentsQuery.refetch(),
        servicesQuery.refetch(),
        pvcQuery.refetch(),
      ]);
    },
  });

  const relatedPods = podsForResult(podsQuery.data ?? [], lastResult);
  const firstRelatedPod = relatedPods[0];
  const firstRelatedContainer = firstRelatedPod?.containers[0]?.name ?? '';
  const deploymentResult = lastBatch?.results.find((item) => item.kind === 'Deployment');
  const serviceResult = lastBatch?.results.find((item) => item.kind === 'Service');
  const pvcResult = lastBatch?.results.find((item) => item.kind === 'PersistentVolumeClaim');
  const relatedDeployment = deploymentsQuery.data?.find((item) => item.name === deploymentResult?.name);
  const relatedService = servicesQuery.data?.find((item) => item.name === serviceResult?.name);
  const relatedPVC = pvcQuery.data?.find((item) => item.name === pvcResult?.name);
  const detailPath = lastResult ? resourcePath(lastResult) : '';
  const diagnoses = buildDiagnosis({
    content,
    pods: relatedPods,
    deployment: relatedDeployment,
    service: relatedService,
    pvc: relatedPVC,
  });

  const podEventsQuery = useQuery({
    queryKey: ['yaml-lab-pod-events', firstRelatedPod?.namespace, firstRelatedPod?.name],
    queryFn: () => getPodEvents(firstRelatedPod?.namespace ?? '', firstRelatedPod?.name ?? ''),
    enabled: Boolean(firstRelatedPod),
  });

  const podDescribeQuery = useQuery({
    queryKey: ['yaml-lab-pod-describe', firstRelatedPod?.namespace, firstRelatedPod?.name],
    queryFn: () => getPodDescribe(firstRelatedPod?.namespace ?? '', firstRelatedPod?.name ?? ''),
    enabled: Boolean(firstRelatedPod),
  });

  const podLogsQuery = useQuery({
    queryKey: ['yaml-lab-pod-logs', firstRelatedPod?.namespace, firstRelatedPod?.name, firstRelatedContainer],
    queryFn: () =>
      getPodLogs(firstRelatedPod?.namespace ?? '', firstRelatedPod?.name ?? '', firstRelatedContainer, 60),
    enabled: Boolean(firstRelatedPod && firstRelatedContainer),
  });

  const applyTemplate = (key: string) => {
    const template = templates.find((item) => item.key === key);
    if (template) {
      setContent(instantiateTemplate(template.content));
      setLastBatch(undefined);
    }
  };

  const handleQuickFix = (diagnosis: DiagnosisItem) => {
    const nextContent = applyQuickFix(content, diagnosis);
    setContent(nextContent);
    setLastBatch(undefined);
    void message.success('已把修复建议写入 YAML 草稿');
  };

  return (
    <main className="space-y-5">
      <section className="rounded-lg border border-slate-200 bg-white px-5 py-4">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <Typography.Text className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">
              YAML LAB
            </Typography.Text>
            <Typography.Title level={3} className="!mb-0 !mt-1">
              YAML 驱动的 Kubernetes 模拟实验室
            </Typography.Title>
          </div>
          <Space wrap>
            <Tag color="blue">{identity.kind || 'Unknown'}</Tag>
            <Tag>{identity.namespace || 'default'}</Tag>
            <Tag>{identity.name || 'unnamed'}</Tag>
          </Space>
        </div>
      </section>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={14}>
          <section className="rounded-lg border border-slate-200 bg-white p-4">
            <div className="mb-3 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <Space>
                <CodeOutlined />
                <Typography.Text strong>输入 Manifest YAML</Typography.Text>
              </Space>
              <Select
                className="w-full md:w-64"
                value={templates.find((item) => item.content === content)?.key}
                placeholder="选择模板"
                onChange={applyTemplate}
                options={templates.map((item) => ({
                  value: item.key,
                  label: item.label,
                }))}
              />
            </div>
            <Input.TextArea
              value={content}
              onChange={(event) => setContent(event.target.value)}
              spellCheck={false}
              className="font-mono"
              autoSize={{ minRows: 24, maxRows: 34 }}
            />
            <div className="mt-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <Typography.Text type="secondary">
                提交后会进入模拟 API Server，并由控制器推导 Pod、Events、Logs、Endpoints 或 PVC 状态。
              </Typography.Text>
              <Button
                type="primary"
                icon={<PlayCircleOutlined />}
                loading={applyMutation.isPending}
                onClick={() => applyMutation.mutate(content)}
              >
                应用到模拟集群
              </Button>
              <Popconfirm
                title="清理当前实验资源?"
                description={`会删除 ${namespace} 中 yaml-/rt- 前缀的实验资源，内置 demo 不受影响。`}
                okText="清理"
                cancelText="取消"
                onConfirm={() => resetMutation.mutate(namespace)}
              >
                <Button danger icon={<ReloadOutlined />} loading={resetMutation.isPending}>
                  清理实验
                </Button>
              </Popconfirm>
            </div>
          </section>
        </Col>

        <Col xs={24} xl={10}>
          <section className="rounded-lg border border-slate-200 bg-white p-4">
            <Space className="mb-3">
              <FileTextOutlined />
              <Typography.Text strong>模拟结果</Typography.Text>
            </Space>

            {applyMutation.isError ? (
              <Alert
                className="mb-4"
                type="error"
                showIcon
                message="YAML 应用失败"
                description={
                  extractErrorMessage(
                    applyMutation.error,
                    '请检查 YAML 格式、kind、metadata.name 或是否已存在同名资源。',
                  )
                }
              />
            ) : null}

            {!lastResult ? (
              <div className="space-y-3">
                {templates.map((template) => (
                  <button
                    key={template.key}
                    className="w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-left transition hover:border-teal-400 hover:bg-teal-50"
                    onClick={() => applyTemplate(template.key)}
                    type="button"
                  >
                    <div className="font-medium text-slate-900">{template.label}</div>
                    <div className="mt-1 text-sm text-slate-500">{template.description}</div>
                  </button>
                ))}
              </div>
            ) : (
              <div className="space-y-4">
                <div className="rounded-md border border-slate-200 bg-slate-50 p-3">
                  <div className="text-sm text-slate-500">创建结果</div>
                  <div className="mt-1 font-medium text-slate-900">
                    {lastBatch?.message}
                  </div>
                  <div className="mt-2 space-y-1 text-sm text-slate-700">
                    {lastBatch?.results.map((result) => (
                      <div key={`${result.kind}-${result.namespace}-${result.name}`}>
                        {result.kind} {result.namespace}/{result.name}: {result.message}
                      </div>
                    ))}
                  </div>
                </div>

                {diagnoses.length > 0 ? (
                  <div className="space-y-2">
                    <Space>
                      <BulbOutlined />
                      <Typography.Text strong>自动诊断</Typography.Text>
                    </Space>
                    {diagnoses.map((item) => (
                      <Alert
                        key={`${item.title}-${item.field}`}
                        type={item.level}
                        showIcon
                        message={
                          <Space wrap>
                            <span>{item.title}</span>
                            <Tag>{item.field}</Tag>
                          </Space>
                        }
                        description={
                          <div className="space-y-1">
                            <div>{item.detail}</div>
                            <div className="text-slate-700">建议：{item.fix}</div>
                            {item.action ? (
                              <Button
                                size="small"
                                icon={<CodeOutlined />}
                                onClick={() => handleQuickFix(item)}
                              >
                                {item.actionLabel ?? '写入修复'}
                              </Button>
                            ) : null}
                          </div>
                        }
                      />
                    ))}
                  </div>
                ) : null}

                {relatedDeployment ? (
                  <div className="rounded-md border border-slate-200 p-3">
                    <Space wrap>
                      <DeploymentUnitOutlined />
                      <Typography.Text strong>Deployment</Typography.Text>
                      <Tag color={statusColor(relatedDeployment.status)}>
                        {relatedDeployment.status}
                      </Tag>
                      <Tag>
                        Ready {relatedDeployment.readyReplicas}/
                        {relatedDeployment.desiredReplicas}
                      </Tag>
                    </Space>
                    <div className="mt-2 text-sm text-slate-600">
                      Images: {relatedDeployment.images.join(', ') || '-'}
                    </div>
                  </div>
                ) : null}

                {relatedService ? (
                  <div className="rounded-md border border-slate-200 p-3">
                    <Space wrap>
                      <Typography.Text strong>Service</Typography.Text>
                      <Tag color={statusColor(relatedService.status)}>{relatedService.status}</Tag>
                      <Tag>Pods {relatedService.podCount}</Tag>
                    </Space>
                    <div className="mt-2 text-sm text-slate-600">
                      Selector: {relatedService.selector.join(', ') || '-'}
                    </div>
                  </div>
                ) : null}

                {relatedPVC ? (
                  <div className="rounded-md border border-slate-200 p-3">
                    <Space wrap>
                      <Typography.Text strong>PVC</Typography.Text>
                      <Tag color={statusColor(relatedPVC.status)}>{relatedPVC.status}</Tag>
                      <Tag>{relatedPVC.storageClass}</Tag>
                    </Space>
                    <div className="mt-2 text-sm text-slate-600">
                      Request: {relatedPVC.requestedStorage}
                    </div>
                  </div>
                ) : null}

                {relatedPods.length > 0 ? (
                  <div className="space-y-2">
                    <Typography.Text type="secondary">关联 Pod</Typography.Text>
                    {relatedPods.map((pod) => (
                      <div
                        key={`${pod.namespace}-${pod.name}`}
                        className="rounded-md border border-slate-200 p-3"
                      >
                        <Space wrap>
                          <Typography.Text strong>{pod.name}</Typography.Text>
                          <Tag color={statusColor(pod.status)}>{pod.status}</Tag>
                          <Tag>
                            Ready {pod.readyContainers}/{pod.totalContainers}
                          </Tag>
                          <Tag>{pod.phase}</Tag>
                        </Space>
                        <div className="mt-2 flex flex-wrap gap-2">
                          <Button
                            size="small"
                            icon={<RightOutlined />}
                            onClick={() =>
                              navigate(
                                `/workloads/pods/${encodeURIComponent(pod.namespace)}/${encodeURIComponent(pod.name)}`,
                              )
                            }
                          >
                            查看 Pod 排障页
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>
                ) : lastResult.kind === 'Deployment' ? (
                  <Alert type="warning" showIcon message="暂未查询到关联 Pod" />
                ) : null}

                {firstRelatedPod ? (
                  <div className="space-y-2 rounded-md border border-slate-200 p-3">
                    <Space>
                      <CodeOutlined />
                      <Typography.Text strong>kubectl 诊断输出</Typography.Text>
                    </Space>
                    <KubectlOutput
                      command={`kubectl get pod -n ${firstRelatedPod.namespace} ${firstRelatedPod.name} -o wide`}
                      content={formatKubectlGetPod(firstRelatedPod)}
                    />
                    <KubectlOutput
                      command={`kubectl get events -n ${firstRelatedPod.namespace} --field-selector involvedObject.name=${firstRelatedPod.name}`}
                      content={formatKubectlEvents(podEventsQuery.data ?? [])}
                    />
                    <KubectlOutput
                      command={`kubectl describe pod -n ${firstRelatedPod.namespace} ${firstRelatedPod.name}`}
                      content={
                        podDescribeQuery.data?.content ??
                        (podDescribeQuery.isLoading ? 'loading describe output...' : 'No describe output.')
                      }
                    />
                    <KubectlOutput
                      command={`kubectl logs -n ${firstRelatedPod.namespace} ${firstRelatedPod.name} -c ${firstRelatedContainer} --tail=60`}
                      content={
                        podLogsQuery.data?.content ??
                        (podLogsQuery.isLoading ? 'loading logs...' : 'No logs available.')
                      }
                    />
                  </div>
                ) : null}

                <Space wrap>
                  {detailPath ? (
                    <Button icon={<RightOutlined />} onClick={() => navigate(detailPath)}>
                      查看资源详情
                    </Button>
                  ) : null}
                  <Button
                    icon={<ReloadOutlined />}
                    onClick={() => {
                      void podsQuery.refetch();
                      void deploymentsQuery.refetch();
                      void servicesQuery.refetch();
                      void pvcQuery.refetch();
                    }}
                  >
                    刷新模拟状态
                  </Button>
                </Space>
              </div>
            )}
          </section>
        </Col>
      </Row>
    </main>
  );
}

function KubectlOutput({ command, content }: { command: string; content: string }) {
  return (
    <div className="overflow-hidden rounded-md border border-slate-200 bg-slate-950">
      <div className="border-b border-slate-800 px-3 py-2 font-mono text-xs text-teal-200">
        $ {command}
      </div>
      <pre className="max-h-72 overflow-auto px-3 py-2 text-xs leading-5 text-slate-100">
        {content}
      </pre>
    </div>
  );
}

function formatKubectlGetPod(pod: PodItem) {
  return [
    'NAME                              READY   STATUS                    RESTARTS   AGE   IP            NODE',
    `${pod.name.padEnd(33)} ${`${pod.readyContainers}/${pod.totalContainers}`.padEnd(7)} ${pod.status.padEnd(25)} ${String(pod.restartCount).padEnd(8)} ${pod.age.padEnd(5)} ${(pod.podIP || '<none>').padEnd(13)} ${pod.nodeName || '<none>'}`,
  ].join('\n');
}

function formatKubectlEvents(events: PodEventItem[]) {
  if (events.length === 0) {
    return 'No events found.';
  }
  const lines = ['LAST SEEN            TYPE      REASON                 COUNT   MESSAGE'];
  for (const event of events) {
    lines.push(
      `${event.lastSeen.padEnd(20)} ${event.type.padEnd(9)} ${event.reason.padEnd(22)} ${String(event.count).padEnd(7)} ${event.message}`,
    );
  }
  return lines.join('\n');
}

function extractErrorMessage(error: unknown, fallback: string) {
  if (!error || typeof error !== 'object') {
    return fallback;
  }

  const maybeError = error as {
    message?: unknown;
    response?: {
      data?: {
        message?: unknown;
      };
    };
  };

  if (
    typeof maybeError.response?.data?.message === 'string' &&
    maybeError.response.data.message.trim() !== ''
  ) {
    return maybeError.response.data.message;
  }
  if (typeof maybeError.message === 'string' && maybeError.message.trim() !== '') {
    return maybeError.message;
  }

  return fallback;
}
