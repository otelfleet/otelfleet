import { AgentService, AgentState as AgentStateEnum, ConfigSyncStatus as ConfigSyncStatusEnum } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type { AgentDescriptionAndStatus, AgentState, ComponentHealth, ConfigSyncStatus } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { ConfigService, ConfigApplicationStatus } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import type { ConfigReference, ConfigAssignmentInfo } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import { useClient } from '../api';
import { useEffect, useState, useCallback, useMemo } from 'react';
import { notifyGRPCError } from '../api/notifications';
import { Badge, Tooltip, Text, Button, Group, Modal, Select, Stack, Paper, ActionIcon } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { Link } from '@tanstack/react-router';
import { CheckCircledIcon, TrashIcon } from '@radix-ui/react-icons';
import {
    Table,
    type ColumnConfig
} from '../components/Table'

function StatusBadge({ state }: { state: AgentState }) {
    const enumStr = AgentStateEnum[state].replace(/AgentState$/i, "");
    const trimmed = typeof enumStr === 'string' && enumStr.toLowerCase().startsWith("agentstate")
        ? enumStr.slice("AgentState".length)
        : enumStr;
    const color = {
        0: 'gray',
        1: 'green',
        2: 'red'
    }[state] ?? 'gray';

    return <Badge color={color} variant="filled" radius="sm">
        {trimmed}
    </Badge>
}

function HealthBadge({ health }: { health?: ComponentHealth }) {
    if (!health) {
        return <Badge color="gray" variant="filled" radius="sm">Unknown</Badge>
    }
    const color = health.healthy ? 'green' : 'red';
    const label = health.healthy ? 'Healthy' : 'Unhealthy';

    return (
        <Badge color={color} variant="filled" radius="sm">
            {label}
        </Badge>
    );
}

function ConfigSyncStatusBadge({ status, reason }: { status?: ConfigSyncStatus; reason?: string }) {
    const statusMap: Record<number, { color: string; label: string }> = {
        [ConfigSyncStatusEnum.UNKNOWN]: { color: 'gray', label: 'Unknown' },
        [ConfigSyncStatusEnum.IN_SYNC]: { color: 'green', label: 'In Sync' },
        [ConfigSyncStatusEnum.OUT_OF_SYNC]: { color: 'yellow', label: 'Out of Sync' },
        [ConfigSyncStatusEnum.APPLYING]: { color: 'blue', label: 'Applying' },
        [ConfigSyncStatusEnum.ERROR]: { color: 'red', label: 'Error' },
    };

    const { color, label } = statusMap[status ?? 0] ?? { color: 'gray', label: 'Unknown' };

    return (
        <Tooltip label={reason} disabled={!reason}>
            <Badge color={color} variant="filled" radius="sm">
                {label}
            </Badge>
        </Tooltip>
    );
}

function AssignedConfigBadge({ assignment }: { assignment?: ConfigAssignmentInfo }) {
    if (!assignment?.configId) {
        return <Text size="sm" c="dimmed">(none)</Text>;
    }

    const statusMap: Record<number, { color: string; label: string }> = {
        [ConfigApplicationStatus.UNSPECIFIED]: { color: 'gray', label: '' },
        [ConfigApplicationStatus.PENDING]: { color: 'yellow', label: 'Pending' },
        [ConfigApplicationStatus.APPLIED]: { color: 'green', label: 'Applied' },
        [ConfigApplicationStatus.FAILED]: { color: 'red', label: 'Failed' },
    };

    const { color, label } = statusMap[assignment.status] ?? { color: 'gray', label: '' };

    return (
        <Tooltip label={assignment.errorMessage} disabled={!assignment.errorMessage}>
            <Group gap="xs" justify="center">
                <Text size="sm" fw={500}>{assignment.configId}</Text>
                {label && <Badge color={color} variant="light" size="xs">{label}</Badge>}
            </Group>
        </Tooltip>
    );
}

export const AgentPage = () => {
    const agentClient = useClient(AgentService);
    const configClient = useClient(ConfigService);

    const [agentsState, setAgentsState] = useState<AgentDescriptionAndStatus[]>([]);
    const [assignments, setAssignments] = useState<Map<string, ConfigAssignmentInfo>>(new Map());
    const [availableConfigs, setAvailableConfigs] = useState<ConfigReference[]>([]);
    const [selectedAgents, setSelectedAgents] = useState<Set<string | number>>(new Set());
    const [selectedConfig, setSelectedConfig] = useState<string | null>(null);
    const [assignModalOpened, { open: openAssignModal, close: closeAssignModal }] = useDisclosure(false);
    const [assigning, setAssigning] = useState(false);
    const [deleteModalOpened, { open: openDeleteModal, close: closeDeleteModal }] = useDisclosure(false);
    const [agentToDelete, setAgentToDelete] = useState<{ id: string; name: string } | null>(null);
    const [deleting, setDeleting] = useState(false);

    const handleListAgents = useCallback(async () => {
        try {
            const response = await agentClient.listAgents({
                withStatus: true,
            });
            setAgentsState(response.agents);
        } catch (error) {
            notifyGRPCError("Failed to list agents", error);
        }
    }, [agentClient]);

    const fetchAssignments = useCallback(async () => {
        try {
            const response = await configClient.listConfigAssignments({});
            const map = new Map<string, ConfigAssignmentInfo>();
            for (const a of response.assignments) {
                map.set(a.agentId, a);
            }
            setAssignments(map);
        } catch (error) {
            notifyGRPCError("Failed to load config assignments", error);
        }
    }, [configClient]);

    const fetchAvailableConfigs = useCallback(async () => {
        try {
            const response = await configClient.listConfigs({});
            setAvailableConfigs(response.configs);
        } catch (error) {
            notifyGRPCError("Failed to load configs", error);
        }
    }, [configClient]);

    const handleBatchAssign = useCallback(async () => {
        if (!selectedConfig || selectedAgents.size === 0) return;
        setAssigning(true);
        try {
            const agentIds = Array.from(selectedAgents) as string[];
            const response = await configClient.batchAssignConfig({ agentIds, configId: selectedConfig });
            notifications.show({
                title: 'Batch Assignment Complete',
                message: `${response.successful} succeeded, ${response.failed} failed`,
                color: response.failed > 0 ? 'yellow' : 'green',
                icon: <CheckCircledIcon />,
            });
            fetchAssignments();
            setSelectedAgents(new Set());
        } catch (error) {
            notifyGRPCError("Failed to batch assign config", error);
        } finally {
            setAssigning(false);
            closeAssignModal();
            setSelectedConfig(null);
        }
    }, [selectedConfig, selectedAgents, configClient, fetchAssignments, closeAssignModal]);

    const handleDeleteAgent = useCallback(async () => {
        if (!agentToDelete) return;
        setDeleting(true);
        try {
            await agentClient.deleteAgent({ agentId: agentToDelete.id });
            notifications.show({
                title: 'Agent Deleted',
                message: `Agent "${agentToDelete.name}" has been deleted`,
                color: 'green',
                icon: <CheckCircledIcon />,
            });
            handleListAgents();
            fetchAssignments();
        } catch (error) {
            notifyGRPCError("Failed to delete agent", error);
        } finally {
            setDeleting(false);
            closeDeleteModal();
            setAgentToDelete(null);
        }
    }, [agentClient, agentToDelete, handleListAgents, fetchAssignments, closeDeleteModal]);

    const confirmDelete = useCallback((agentId: string, agentName: string) => {
        setAgentToDelete({ id: agentId, name: agentName });
        openDeleteModal();
    }, [openDeleteModal]);

    useEffect(() => {
        handleListAgents();
        fetchAssignments();
    }, [handleListAgents, fetchAssignments]);

    useEffect(() => {
        if (assignModalOpened) {
            fetchAvailableConfigs();
        }
    }, [assignModalOpened, fetchAvailableConfigs]);

    const agentColumns = useMemo<ColumnConfig<AgentDescriptionAndStatus>[]>(() => [
        {
            key: 'name',
            label: 'Name',
            visible: true,
            render: (_: unknown, row: AgentDescriptionAndStatus) => {
                return <Text fw={500}>{row.agent?.friendlyName || 'Unknown'}</Text>
            }
        },
        {
            key: 'connection',
            label: 'Connection',
            visible: true,
            render: (_: unknown, row: AgentDescriptionAndStatus) => {
                return <StatusBadge state={row.status?.state ?? 0} />
            }
        },
        {
            key: 'health',
            label: 'Health',
            visible: true,
            render: (_: unknown, row: AgentDescriptionAndStatus) => {
                return <HealthBadge health={row.status?.health} />
            }
        },
        {
            key: 'configStatus',
            label: 'Config Sync',
            visible: true,
            render: (_: unknown, row: AgentDescriptionAndStatus) => {
                return <ConfigSyncStatusBadge
                    status={row.status?.configSyncStatus}
                    reason={row.status?.configSyncReason}
                />
            }
        },
        {
            key: 'assignedConfig',
            label: 'Assigned Config',
            visible: true,
            render: (_: unknown, row: AgentDescriptionAndStatus) => {
                const assignment = assignments.get(row.agent?.id ?? '');
                return <AssignedConfigBadge assignment={assignment} />
            }
        },
        {
            key: 'actions',
            label: '',
            visible: true,
            render: (_: unknown, row: AgentDescriptionAndStatus) => {
                if (!row.agent?.id) return null;
                return (
                    <Group gap="xs" justify="center">
                        <Link to="/agents/$agentId" params={{ agentId: row.agent.id }}>
                            <Button size="xs" variant="light">
                                Details
                            </Button>
                        </Link>
                        <ActionIcon
                            color="red"
                            variant="subtle"
                            size="lg"
                            onClick={() => confirmDelete(row.agent!.id, row.agent!.friendlyName || 'Unknown')}
                            title="Delete agent"
                        >
                            <TrashIcon width={18} height={18} />
                        </ActionIcon>
                    </Group>
                );
            }
        },
    ], [assignments, confirmDelete]);

    return (
        <>
            {selectedAgents.size > 0 && (
                <Paper p="sm" mb="md" withBorder>
                    <Group justify="space-between">
                        <Text size="sm" fw={500}>
                            {selectedAgents.size} agent{selectedAgents.size > 1 ? 's' : ''} selected
                        </Text>
                        <Group gap="xs">
                            <Button size="xs" variant="light" onClick={openAssignModal}>
                                Assign Config
                            </Button>
                            <Button size="xs" variant="subtle" onClick={() => setSelectedAgents(new Set())}>
                                Clear Selection
                            </Button>
                        </Group>
                    </Group>
                </Paper>
            )}

            <Table<AgentDescriptionAndStatus>
                title="OpenTelemetry Collector agents"
                data={agentsState}
                columns={agentColumns}
                rowKey={(row) => row.agent?.id ?? ''}
                selectable
                selectedKeys={selectedAgents}
                onSelectionChange={setSelectedAgents}
                expandedContent={(row) => {
                    const error = row.status?.health?.lastError;
                    if (!error) return null;
                    return (
                        <Text size="sm" c="red">
                            {error}
                        </Text>
                    );
                }}
            />

            {/* Batch Assign Config Modal */}
            <Modal opened={assignModalOpened} onClose={closeAssignModal} title="Assign Config to Selected Agents">
                <Stack gap="md">
                    <Text size="sm">
                        Assign a configuration to {selectedAgents.size} selected agent{selectedAgents.size > 1 ? 's' : ''}.
                    </Text>
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
                        <Button onClick={handleBatchAssign} loading={assigning} disabled={!selectedConfig}>
                            Assign to {selectedAgents.size} Agent{selectedAgents.size > 1 ? 's' : ''}
                        </Button>
                    </Group>
                </Stack>
            </Modal>

            {/* Delete Agent Confirmation Modal */}
            <Modal opened={deleteModalOpened} onClose={closeDeleteModal} title="Delete Agent" centered>
                <Stack gap="md">
                    <Text size="sm">
                        Are you sure you want to delete agent <Text span fw={700}>{agentToDelete?.name}</Text>?
                    </Text>
                    <Text size="sm" c="dimmed">
                        This will permanently remove the agent and all its associated data including
                        health status, configuration state, and connection history. This action cannot be undone.
                    </Text>
                    <Group justify="flex-end" mt="md">
                        <Button variant="default" onClick={closeDeleteModal}>Cancel</Button>
                        <Button color="red" onClick={handleDeleteAgent} loading={deleting}>
                            Delete Agent
                        </Button>
                    </Group>
                </Stack>
            </Modal>
        </>
    );
}