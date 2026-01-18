import { useEffect, useState, useCallback } from 'react';
import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import {
    AgentService,
    AgentState as AgentStateEnum,
    ConfigSyncStatus as ConfigSyncStatusEnum,
} from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type {
    AgentDescription,
    AgentStatus,
    ComponentHealth,
    KeyValue,
    AnyValue,
} from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import {
    ConfigService,
    ConfigSource,
} from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import type {
    ConfigReference,
    GetAgentConfigResponse,
} from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import {
    Tabs,
    Paper,
    Title,
    Text,
    Badge,
    Group,
    Stack,
    Table,
    Loader,
    Center,
    Alert,
    Box,
    Button,
    Modal,
    Select,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { AlertCircle } from 'react-feather';
import { CheckCircledIcon } from '@radix-ui/react-icons';
import { Editor } from '../components/Editor';

interface AgentDetailPageProps {
    agentId: string;
}

export function AgentDetailPage({ agentId }: AgentDetailPageProps) {
    const agentClient = useClient(AgentService);
    const configClient = useClient(ConfigService);
    const [agent, setAgent] = useState<AgentDescription | null>(null);
    const [status, setStatus] = useState<AgentStatus | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    // Config assignment state
    const [configAssignment, setConfigAssignment] = useState<GetAgentConfigResponse | null>(null);
    const [availableConfigs, setAvailableConfigs] = useState<ConfigReference[]>([]);
    const [selectedConfig, setSelectedConfig] = useState<string | null>(null);
    const [assignModalOpened, { open: openAssignModal, close: closeAssignModal }] = useDisclosure(false);
    const [unassignModalOpened, { open: openUnassignModal, close: closeUnassignModal }] = useDisclosure(false);
    const [assigning, setAssigning] = useState(false);

    const fetchConfigAssignment = useCallback(async () => {
        try {
            const response = await configClient.getAgentConfig({ agentId });
            setConfigAssignment(response);
        } catch {
            // Agent might not have a config assigned, which is fine
            setConfigAssignment(null);
        }
    }, [agentId, configClient]);

    const fetchAvailableConfigs = useCallback(async () => {
        try {
            const response = await configClient.listConfigs({});
            setAvailableConfigs(response.configs);
        } catch (err) {
            notifyGRPCError('Failed to load configs', err);
        }
    }, [configClient]);

    const handleAssignConfig = useCallback(async () => {
        if (!selectedConfig) return;
        setAssigning(true);
        try {
            const response = await configClient.assignConfig({ agentId, configId: selectedConfig });
            if (response.success) {
                notifications.show({
                    title: 'Config assigned',
                    message: `Successfully assigned config "${selectedConfig}" to agent`,
                    icon: <CheckCircledIcon />,
                });
                fetchConfigAssignment();
            } else {
                notifications.show({
                    title: 'Assignment failed',
                    message: response.message,
                    color: 'red',
                });
            }
        } catch (err) {
            notifyGRPCError('Failed to assign config', err);
        } finally {
            setAssigning(false);
            closeAssignModal();
            setSelectedConfig(null);
        }
    }, [agentId, selectedConfig, configClient, fetchConfigAssignment, closeAssignModal]);

    const handleUnassignConfig = useCallback(async () => {
        try {
            const response = await configClient.unassignConfig({ agentId });
            if (response.success) {
                notifications.show({
                    title: 'Config unassigned',
                    message: 'Successfully removed config from agent',
                    icon: <CheckCircledIcon />,
                });
                fetchConfigAssignment();
            }
        } catch (err) {
            notifyGRPCError('Failed to unassign config', err);
        } finally {
            closeUnassignModal();
        }
    }, [agentId, configClient, fetchConfigAssignment, closeUnassignModal]);

    useEffect(() => {
        const fetchAgentData = async () => {
            setLoading(true);
            setError(null);
            try {
                const [agentResponse, statusResponse] = await Promise.all([
                    agentClient.getAgent({ agentId }),
                    agentClient.status({ agentId }),
                ]);
                setAgent(agentResponse.agent ?? null);
                setStatus(statusResponse.status ?? null);
            } catch (err) {
                notifyGRPCError('Failed to fetch agent details', err);
                setError('Failed to load agent details');
            } finally {
                setLoading(false);
            }
        };

        fetchAgentData();
        fetchConfigAssignment();
    }, [agentId, agentClient, fetchConfigAssignment]);

    useEffect(() => {
        if (assignModalOpened) {
            fetchAvailableConfigs();
        }
    }, [assignModalOpened, fetchAvailableConfigs]);

    if (loading) {
        return (
            <Center style={{ height: '100%', minHeight: 400 }}>
                <Loader size="lg" />
            </Center>
        );
    }

    if (error) {
        return (
            <Alert color="red" title="Error" icon={<AlertCircle size={16} />}>
                {error}
            </Alert>
        );
    }

    return (
        <>
            <Stack gap="md" style={{ height: '100%' }}>
                <AgentHeader agent={agent} status={status} />
                <ConfigAssignmentSection
                    assignment={configAssignment}
                    onAssign={openAssignModal}
                    onUnassign={openUnassignModal}
                />
                <Tabs defaultValue="health" style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                    <Tabs.List>
                        <Tabs.Tab value="health">Health</Tabs.Tab>
                        <Tabs.Tab value="details">Details</Tabs.Tab>
                        <Tabs.Tab value="config">Effective Config</Tabs.Tab>
                    </Tabs.List>

                    <Tabs.Panel value="health" pt="md" style={{ flex: 1 }}>
                        <HealthTab health={status?.health} />
                    </Tabs.Panel>

                    <Tabs.Panel value="details" pt="md" style={{ flex: 1 }}>
                        <DetailsTab agent={agent} />
                    </Tabs.Panel>

                    <Tabs.Panel value="config" pt="md" style={{ flex: 1, minHeight: 400 }}>
                        <EffectiveConfigTab status={status} />
                    </Tabs.Panel>
                </Tabs>
            </Stack>

            {/* Assign Config Modal */}
            <Modal opened={assignModalOpened} onClose={closeAssignModal} title="Assign Config">
                <Stack gap="md">
                    <Select
                        label="Select Config"
                        placeholder="Choose a configuration"
                        data={availableConfigs.map(c => ({ value: c.id, label: c.id }))}
                        value={selectedConfig}
                        onChange={setSelectedConfig}
                        searchable
                    />
                    <Group justify="flex-end" mt="md">
                        <Button variant="default" onClick={closeAssignModal}>Cancel</Button>
                        <Button onClick={handleAssignConfig} loading={assigning} disabled={!selectedConfig}>
                            Assign
                        </Button>
                    </Group>
                </Stack>
            </Modal>

            {/* Unassign Config Modal */}
            <Modal opened={unassignModalOpened} onClose={closeUnassignModal} title="Unassign Config">
                <Text>Are you sure you want to remove the config assignment from this agent?</Text>
                <Text size="sm" c="dimmed" mt="xs">
                    The agent will fall back to the default configuration.
                </Text>
                <Group justify="flex-end" mt="md">
                    <Button variant="default" onClick={closeUnassignModal}>Cancel</Button>
                    <Button color="red" onClick={handleUnassignConfig}>Unassign</Button>
                </Group>
            </Modal>
        </>
    );
}

function AgentHeader({ agent, status }: { agent: AgentDescription | null; status: AgentStatus | null }) {
    const stateColor = {
        0: 'gray',
        1: 'green',
        2: 'red',
    }[status?.state ?? 0] ?? 'gray';

    const stateLabel = AgentStateEnum[status?.state ?? 0]?.replace(/^AGENT_STATE_/, '') ?? 'UNKNOWN';

    const configSyncStatusMap: Record<number, { color: string; label: string }> = {
        [ConfigSyncStatusEnum.UNKNOWN]: { color: 'gray', label: 'Unknown' },
        [ConfigSyncStatusEnum.IN_SYNC]: { color: 'green', label: 'In Sync' },
        [ConfigSyncStatusEnum.OUT_OF_SYNC]: { color: 'yellow', label: 'Out of Sync' },
        [ConfigSyncStatusEnum.APPLYING]: { color: 'blue', label: 'Applying' },
        [ConfigSyncStatusEnum.ERROR]: { color: 'red', label: 'Error' },
    };

    const configStatus = configSyncStatusMap[status?.configSyncStatus ?? 0] ?? { color: 'gray', label: 'Unknown' };

    return (
        <Paper p="md" withBorder>
            <Group justify="space-between" align="flex-start">
                <Stack gap="xs">
                    <Title order={2}>{agent?.friendlyName || 'Unknown Agent'}</Title>
                    <Text size="sm" c="dimmed">ID: {agent?.id || 'N/A'}</Text>
                </Stack>
                <Group gap="sm">
                    <Badge color={stateColor} variant="filled" size="lg">
                        {stateLabel}
                    </Badge>
                    <Badge color={status?.health?.healthy ? 'green' : 'red'} variant="filled" size="lg">
                        {status?.health?.healthy ? 'Healthy' : 'Unhealthy'}
                    </Badge>
                    <Badge color={configStatus.color} variant="filled" size="lg">
                        Config Sync: {configStatus.label}
                    </Badge>
                </Group>
            </Group>
        </Paper>
    );
}

function ConfigAssignmentSection({
    assignment,
    onAssign,
    onUnassign,
}: {
    assignment: GetAgentConfigResponse | null;
    onAssign: () => void;
    onUnassign: () => void;
}) {
    const formatDate = (timestamp?: { seconds?: bigint; nanos?: number }) => {
        if (!timestamp?.seconds) return 'N/A';
        return new Date(Number(timestamp.seconds) * 1000).toLocaleString();
    };

    const sourceLabel = {
        [ConfigSource.UNSPECIFIED]: 'Unknown',
        [ConfigSource.DEFAULT]: 'Default',
        [ConfigSource.BOOTSTRAP]: 'Bootstrap',
        [ConfigSource.MANUAL]: 'Manual',
    }[assignment?.source ?? ConfigSource.UNSPECIFIED];

    const sourceColor = {
        [ConfigSource.UNSPECIFIED]: 'gray',
        [ConfigSource.DEFAULT]: 'blue',
        [ConfigSource.BOOTSTRAP]: 'violet',
        [ConfigSource.MANUAL]: 'green',
    }[assignment?.source ?? ConfigSource.UNSPECIFIED];

    return (
        <Paper p="md" withBorder>
            <Group justify="space-between" align="flex-start">
                <Stack gap="xs">
                    <Title order={4}>Config Assignment</Title>
                    {assignment?.configId ? (
                        <>
                            <Group gap="lg">
                                <Stack gap={2}>
                                    <Text size="sm" c="dimmed">Config</Text>
                                    <Text fw={500}>{assignment.configId}</Text>
                                </Stack>
                                <Stack gap={2}>
                                    <Text size="sm" c="dimmed">Source</Text>
                                    <Badge color={sourceColor} variant="light">{sourceLabel}</Badge>
                                </Stack>
                                <Stack gap={2}>
                                    <Text size="sm" c="dimmed">Assigned At</Text>
                                    <Text size="sm">{formatDate(assignment.assignedAt)}</Text>
                                </Stack>
                            </Group>
                        </>
                    ) : (
                        <Text c="dimmed">No config assigned - using default configuration</Text>
                    )}
                </Stack>
                <Group gap="xs">
                    <Button variant="light" size="sm" onClick={onAssign}>
                        {assignment?.configId ? 'Change Config' : 'Assign Config'}
                    </Button>
                    {assignment?.configId && (
                        <Button variant="light" color="red" size="sm" onClick={onUnassign}>
                            Unassign
                        </Button>
                    )}
                </Group>
            </Group>
        </Paper>
    );
}

function HealthTab({ health }: { health?: ComponentHealth }) {
    if (!health) {
        return (
            <Alert color="gray" title="No Health Data">
                No health information available for this agent.
            </Alert>
        );
    }

    return (
        <Stack gap="md">
            <HealthOverview health={health} />
            <ComponentHealthTable health={health} />
        </Stack>
    );
}

function HealthOverview({ health }: { health: ComponentHealth }) {
    const formatTime = (nanos: bigint) => {
        if (!nanos) return 'N/A';
        const ms = Number(nanos) / 1_000_000;
        return new Date(ms).toLocaleString();
    };

    return (
        <Paper p="md" withBorder>
            <Title order={4} mb="md">Health Overview</Title>
            <Group gap="xl">
                <Stack gap="xs">
                    <Text size="sm" c="dimmed">Status</Text>
                    <Badge color={health.healthy ? 'green' : 'red'} variant="filled" size="lg">
                        {health.healthy ? 'Healthy' : 'Unhealthy'}
                    </Badge>
                </Stack>
                <Stack gap="xs">
                    <Text size="sm" c="dimmed">Status Message</Text>
                    <Text>{health.status || 'N/A'}</Text>
                </Stack>
                <Stack gap="xs">
                    <Text size="sm" c="dimmed">Start Time</Text>
                    <Text>{formatTime(health.startTimeUnixNano)}</Text>
                </Stack>
                <Stack gap="xs">
                    <Text size="sm" c="dimmed">Last Status Update</Text>
                    <Text>{formatTime(health.statusTimeUnixNano)}</Text>
                </Stack>
                {health.lastError && (
                    <Stack gap="xs">
                        <Text size="sm" c="dimmed">Last Error</Text>
                        <Text c="red">{health.lastError}</Text>
                    </Stack>
                )}
            </Group>
        </Paper>
    );
}

function DetailsTab({ agent }: { agent: AgentDescription | null }) {
    if (!agent) {
        return (
            <Alert color="gray" title="No Agent Data">
                No agent information available.
            </Alert>
        );
    }

    return (
        <Stack gap="md">
            <AttributesTable
                title="Identifying Attributes"
                description="Attributes that uniquely identify the agent (e.g., service.name, service.instance.id)"
                attributes={agent.identifyingAttributes}
            />
            <AttributesTable
                title="Non-Identifying Attributes"
                description="Attributes that describe the agent's environment (e.g., os.type, host.arch)"
                attributes={agent.nonIdentifyingAttributes}
            />
            <CapabilitiesCard capabilities={agent.capabilities} />
        </Stack>
    );
}

function formatAnyValue(value: AnyValue | undefined): string {
    if (!value || value.value.case === undefined) {
        return 'N/A';
    }

    switch (value.value.case) {
        case 'stringValue':
            return value.value.value;
        case 'boolValue':
            return value.value.value ? 'true' : 'false';
        case 'intValue':
            return value.value.value.toString();
        case 'doubleValue':
            return value.value.value.toString();
        case 'bytesValue':
            return `<bytes: ${value.value.value.length} bytes>`;
        case 'arrayValue':
            return `[${value.value.value.values.map(v => formatAnyValue(v)).join(', ')}]`;
        case 'kvlistValue':
            return `{${value.value.value.values.map(kv => `${kv.key}: ${formatAnyValue(kv.value)}`).join(', ')}}`;
        default:
            return 'N/A';
    }
}

function AttributesTable({ title, description, attributes }: {
    title: string;
    description: string;
    attributes: KeyValue[];
}) {
    return (
        <Paper p="md" withBorder>
            <Title order={4} mb="xs">{title}</Title>
            <Text size="sm" c="dimmed" mb="md">{description}</Text>
            {attributes.length === 0 ? (
                <Text size="sm" c="dimmed">No attributes available</Text>
            ) : (
                <Table striped highlightOnHover>
                    <Table.Thead>
                        <Table.Tr>
                            <Table.Th>Key</Table.Th>
                            <Table.Th>Value</Table.Th>
                        </Table.Tr>
                    </Table.Thead>
                    <Table.Tbody>
                        {attributes.map((attr) => (
                            <Table.Tr key={attr.key}>
                                <Table.Td>
                                    <Text size="sm" fw={500}>{attr.key}</Text>
                                </Table.Td>
                                <Table.Td>
                                    <Text size="sm" style={{ fontFamily: 'monospace' }}>
                                        {formatAnyValue(attr.value)}
                                    </Text>
                                </Table.Td>
                            </Table.Tr>
                        ))}
                    </Table.Tbody>
                </Table>
            )}
        </Paper>
    );
}

function CapabilitiesCard({ capabilities }: { capabilities: string[] }) {
    return (
        <Paper p="md" withBorder>
            <Title order={4} mb="xs">Capabilities</Title>
            <Text size="sm" c="dimmed" mb="md">Features and capabilities supported by this agent</Text>
            {capabilities.length === 0 ? (
                <Text size="sm" c="dimmed">No capabilities reported</Text>
            ) : (
                <Group gap="sm">
                    {capabilities.map((capability) => (
                        <Badge key={capability} variant="light" color="blue" size="lg">
                            {capability}
                        </Badge>
                    ))}
                </Group>
            )}
        </Paper>
    );
}

function ComponentHealthTable({ health }: { health: ComponentHealth }) {
    const componentMap = health.componentHealthMap;

    if (!componentMap || Object.keys(componentMap).length === 0) {
        return null;
    }

    return (
        <Paper p="md" withBorder>
            <Title order={4} mb="md">Component Health</Title>
            <Table striped highlightOnHover>
                <Table.Thead>
                    <Table.Tr>
                        <Table.Th>Component</Table.Th>
                        <Table.Th>Status</Table.Th>
                        <Table.Th>Health</Table.Th>
                        <Table.Th>Last Error</Table.Th>
                    </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                    <ComponentRows componentMap={componentMap} depth={0} />
                </Table.Tbody>
            </Table>
        </Paper>
    );
}

function ComponentRows({ componentMap, depth, parentName = '' }: {
    componentMap: { [key: string]: ComponentHealth };
    depth: number;
    parentName?: string;
}) {
    return (
        <>
            {Object.entries(componentMap).map(([name, component]) => {
                const fullName = parentName ? `${parentName} > ${name}` : name;
                const hasChildren = component.componentHealthMap && Object.keys(component.componentHealthMap).length > 0;

                return (
                    <ComponentRow
                        key={fullName}
                        name={name}
                        component={component}
                        depth={depth}
                        hasChildren={hasChildren}
                    />
                );
            })}
        </>
    );
}

function ComponentRow({ name, component, depth, hasChildren }: {
    name: string;
    component: ComponentHealth;
    depth: number;
    hasChildren: boolean;
}) {
    const [expanded, setExpanded] = useState(true);

    return (
        <>
            <Table.Tr
                onClick={hasChildren ? () => setExpanded(!expanded) : undefined}
                style={hasChildren ? { cursor: 'pointer' } : undefined}
            >
                <Table.Td>
                    <Box style={{ paddingLeft: depth * 20 }}>
                        <Group gap="xs">
                            {hasChildren && (
                                <Text size="sm" c="dimmed">
                                    {expanded ? '▼' : '▶'}
                                </Text>
                            )}
                            <Text fw={depth === 0 ? 600 : 400}>{name}</Text>
                        </Group>
                    </Box>
                </Table.Td>
                <Table.Td>
                    <Text size="sm">{component.status || 'N/A'}</Text>
                </Table.Td>
                <Table.Td>
                    <Badge color={component.healthy ? 'green' : 'red'} variant="filled" size="sm">
                        {component.healthy ? 'Healthy' : 'Unhealthy'}
                    </Badge>
                </Table.Td>
                <Table.Td>
                    <Text size="sm" c={component.lastError ? 'red' : 'dimmed'}>
                        {component.lastError || '-'}
                    </Text>
                </Table.Td>
            </Table.Tr>
            {hasChildren && expanded && (
                <ComponentRows
                    componentMap={component.componentHealthMap}
                    depth={depth + 1}
                />
            )}
        </>
    );
}

function EffectiveConfigTab({ status }: { status: AgentStatus | null }) {
    const configMap = status?.effectiveConfig?.configMap?.configMap;

    if (!configMap || Object.keys(configMap).length === 0) {
        return (
            <Alert color="gray" title="No Configuration">
                No effective configuration available for this agent.
            </Alert>
        );
    }

    // Get the first config file (usually there's only one)
    const [configName, configFile] = Object.entries(configMap)[0];
    const configContent = configFile?.body
        ? new TextDecoder().decode(configFile.body)
        : '';

    return (
        <Paper p="md" withBorder style={{ display: 'flex', flexDirection: 'column' }}>
            <Group justify="space-between" mb="md">
                <Title order={4}>Effective Configuration</Title>
                <Text size="sm" c="dimmed">{configName}</Text>
            </Group>
            <Editor
                defaultConfig={configContent}
                readOnly
                height={500}
            />
        </Paper>
    );
}
