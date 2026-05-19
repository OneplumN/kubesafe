import {
  CheckCircleOutlined,
  CodeOutlined,
  ReloadOutlined,
  RightOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { App, Button, Card, Col, Collapse, Empty, Row, Space, Tag, Typography } from 'antd';
import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';

import {
  getSimulationScenarios,
  resetSimulationScenario,
  type SimulationScenario,
} from '../services/cluster';

function difficultyColor(value: string) {
  switch (value.toLowerCase()) {
    case 'beginner':
      return 'green';
    case 'intermediate':
      return 'orange';
    case 'advanced':
      return 'red';
    default:
      return 'blue';
  }
}

function categoryColor(value: string) {
  switch (value.toLowerCase()) {
    case 'network':
      return 'geekblue';
    case 'storage':
      return 'purple';
    case 'workload':
      return 'cyan';
    default:
      return 'default';
  }
}

export function SimulationScenariosPage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const scenariosQuery = useQuery({
    queryKey: ['simulation-scenarios'],
    queryFn: getSimulationScenarios,
  });

  const resetMutation = useMutation({
    mutationFn: resetSimulationScenario,
    onSuccess: async (scenario) => {
      message.success(`${scenario.title} 已重置`);
      await queryClient.invalidateQueries({ queryKey: ['simulation-scenarios'] });
      await queryClient.invalidateQueries({ queryKey: ['deployments'] });
      await queryClient.invalidateQueries({ queryKey: ['pods'] });
      await queryClient.invalidateQueries({ queryKey: ['services'] });
      await queryClient.invalidateQueries({ queryKey: ['endpoints'] });
      await queryClient.invalidateQueries({ queryKey: ['persistentvolumeclaims'] });
    },
  });

  const scenarios = scenariosQuery.data ?? [];
  const stats = useMemo(() => {
    const completed = scenarios.filter((item) => item.completed).length;
    return {
      total: scenarios.length,
      completed,
      active: scenarios.length - completed,
    };
  }, [scenarios]);

  return (
    <main className="space-y-5">
      <section className="rounded-lg border border-slate-200 bg-white px-5 py-4">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <Typography.Text className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">
              Simulation Training
            </Typography.Text>
            <Typography.Title level={3} className="!mb-0 !mt-1">
              Kubernetes 故障训练场景
            </Typography.Title>
          </div>
          <Space wrap>
            <Tag color="blue">Total {stats.total}</Tag>
            <Tag color="green">Completed {stats.completed}</Tag>
            <Tag color="orange">Active {stats.active}</Tag>
            <Button
              icon={<ReloadOutlined />}
              onClick={() => void scenariosQuery.refetch()}
              loading={scenariosQuery.isFetching}
            >
              刷新
            </Button>
          </Space>
        </div>
      </section>

      {scenarios.length === 0 && !scenariosQuery.isFetching ? (
        <Card>
          <Empty description="当前会话没有可用训练场景" />
        </Card>
      ) : (
        <Row gutter={[16, 16]}>
          {scenarios.map((scenario: SimulationScenario) => (
            <Col xs={24} lg={12} xl={8} key={scenario.id}>
              <Card
                className="h-full"
                title={
                  <Space direction="vertical" size={2}>
                    <Space wrap>
                      <Tag color={scenario.completed ? 'green' : 'orange'}>
                        {scenario.completed ? (
                          <CheckCircleOutlined />
                        ) : (
                          <WarningOutlined />
                        )}{' '}
                        {scenario.completed ? 'Completed' : 'Active'}
                      </Tag>
                      <Tag color={categoryColor(scenario.category)}>{scenario.category}</Tag>
                      <Tag color={difficultyColor(scenario.difficulty)}>
                        {scenario.difficulty}
                      </Tag>
                    </Space>
                    <Typography.Text strong>{scenario.title}</Typography.Text>
                  </Space>
                }
              >
                <div className="space-y-4">
                  <div>
                    <Typography.Text type="secondary">当前状态</Typography.Text>
                    <div className="mt-1 text-sm text-slate-800">{scenario.status}</div>
                  </div>
                  <div>
                    <Typography.Text type="secondary">现象</Typography.Text>
                    <div className="mt-1 text-sm text-slate-800">{scenario.symptom}</div>
                  </div>
                  <div>
                    <Typography.Text type="secondary">目标</Typography.Text>
                    <div className="mt-1 text-sm text-slate-800">{scenario.goal}</div>
                  </div>
                  <div>
                    <Typography.Text type="secondary">排查线索</Typography.Text>
                    <div className="mt-1 text-sm text-slate-800">{scenario.diagnosis}</div>
                  </div>
                  <div>
                    <Typography.Text type="secondary">修复提示</Typography.Text>
                    <div className="mt-1 text-sm text-slate-800">{scenario.fixHint}</div>
                  </div>
                  <Space wrap>
                    {scenario.entryResources.map((resource) => (
                      <Button
                        key={`${resource.kind}-${resource.namespace}-${resource.name}`}
                        size="small"
                        icon={<RightOutlined />}
                        onClick={() => navigate(resource.path)}
                      >
                        {resource.kind} {resource.namespace ? `${resource.namespace}/` : ''}
                        {resource.name}
                      </Button>
                    ))}
                  </Space>
                  {scenario.commands.length > 0 ? (
                    <Collapse
                      size="small"
                      bordered={false}
                      className="bg-slate-50"
                      items={[
                        {
                          key: 'commands',
                          label: (
                            <Space size={6}>
                              <CodeOutlined />
                              <span>kubectl 诊断输出</span>
                              <Tag>{scenario.commands.length}</Tag>
                            </Space>
                          ),
                          children: (
                            <div className="space-y-3">
                              {scenario.commands.map((command) => (
                                <div
                                  key={`${scenario.id}-${command.command}`}
                                  className="rounded-md border border-slate-200 bg-white"
                                >
                                  <div className="border-b border-slate-100 px-3 py-2">
                                    <Typography.Text strong className="text-xs">
                                      {command.title}
                                    </Typography.Text>
                                    <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words font-mono text-xs text-teal-700">
                                      $ {command.command}
                                    </pre>
                                  </div>
                                  <pre className="max-h-56 overflow-auto whitespace-pre-wrap break-words px-3 py-2 font-mono text-xs leading-5 text-slate-800">
                                    {command.output}
                                  </pre>
                                </div>
                              ))}
                            </div>
                          ),
                        },
                      ]}
                    />
                  ) : null}
                  <Button
                    block
                    icon={<ReloadOutlined />}
                    loading={resetMutation.isPending && resetMutation.variables === scenario.id}
                    onClick={() => resetMutation.mutate(scenario.id)}
                  >
                    重置本场景
                  </Button>
                </div>
              </Card>
            </Col>
          ))}
        </Row>
      )}
    </main>
  );
}
