import { useState, useEffect, useCallback, useMemo } from 'react';
import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import {
    ConfigService,
    ConfigSource,
    ConfigApplicationStatus,
} from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import type {
    ConfigAssignmentInfo,
    ConfigReference,
} from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import {
    Group,
    Badge,
    Button,
    Text,
    Modal,
    Select,
    Stack,
    ActionIcon,
    Tooltip,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { Link } from '@tanstack/react-router';
import { CheckCircledIcon, Cross2Icon, ExternalLinkIcon } from '@radix-ui/react-icons';
import { Table, type ColumnConfig } from '../components/Table';
import { LabelSelector } from '../components/LabelSelector';

function ConfigSourceBadge({ source }: { source: ConfigSource }) {
    const config = {
        [ConfigSource.UNSPECIFIED]: { color: 'gray', label: 'Unknown' },
        [ConfigSource.DEFAULT]: { color: 'blue', label: 'Default' },
        [ConfigSource.BOOTSTRAP]: { color: 'violet', label: 'Bootstrap' },
        [ConfigSource.MANUAL]: { color: 'green', label: 'Manual' },
    }[source] ?? { color: 'gray', label: 'Unknown' };

    return <Badge color={config.color} variant="light">{config.label}</Badge>;
}

function ApplicationStatusBadge({ status, errorMessage }: { status: ConfigApplicationStatus; errorMessage?: string }) {
    const config = {
        [ConfigApplicationStatus.UNSPECIFIED]: { color: 'gray', label: 'Unknown' },
        [ConfigApplicationStatus.PENDING]: { color: 'yellow', label: 'Pending' },
        [ConfigApplicationStatus.APPLIED]: { color: 'green', label: 'Applied' },
        [ConfigApplicationStatus.FAILED]: { color: 'red', label: 'Failed' },
    }[status] ?? { color: 'gray', label: 'Unknown' };

    return (
        <Tooltip label={errorMessage} disabled={!errorMessage}>
            <Badge color={config.color} variant="filled">{config.label}</Badge>
        </Tooltip>
    );
}

export function ListAssignmentsPage() {
    const configClient = useClient(ConfigService);

    const [assignments, setAssignments] = useState<ConfigAssignmentInfo[]>([]);
    const [availableConfigs, setAvailableConfigs] = useState<ConfigReference[]>([]);
    const [configFilter, setConfigFilter] = useState<string | null>(null);
    const [statusFilter, setStatusFilter] = useState<string | null>(null);
    const [sourceFilter, setSourceFilter] = useState<string | null>(null);
    const [selectedAssignments, setSelectedAssignments] = useState<Set<string | number>>(new Set());
    const [unassignModalOpened, { open: openUnassignModal, close: closeUnassignModal }] = useDisclosure(false);
    const [assignmentToUnassign, setAssignmentToUnassign] = useState<string | null>(null);
    const [bulkUnassignModalOpened, { open: openBulkUnassignModal, close: closeBulkUnassignModal }] = useDisclosure(false);

    const fetchAssignments = useCallback(async () => {
        try {
            const response = await configClient.listConfigAssignments({
                configId: configFilter || undefined,
            });
            setAssignments(response.assignments);
        } catch (error) {
            notifyGRPCError('Failed to load assignments', error);
        }
    }, [configClient, configFilter]);

    const fetchConfigs = useCallback(async () => {
        try {
            const response = await configClient.listConfigs({});
            setAvailableConfigs(response.configs);
        } catch (error) {
            notifyGRPCError('Failed to load configs', error);
        }
    }, [configClient]);

    const handleUnassign = useCallback(async () => {
        if (!assignmentToUnassign) return;
        try {
            const response = await configClient.unassignConfig({ agentId: assignmentToUnassign });
            if (response.success) {
                notifications.show({
                    title: 'Config unassigned',
                    message: 'Successfully removed config from agent',
                    icon: <CheckCircledIcon />,
                });
                fetchAssignments();
            }
        } catch (error) {
            notifyGRPCError('Failed to unassign config', error);
        } finally {
            closeUnassignModal();
            setAssignmentToUnassign(null);
        }
    }, [assignmentToUnassign, configClient, fetchAssignments, closeUnassignModal]);

    const handleBulkUnassign = useCallback(async () => {
        const agentIds = Array.from(selectedAssignments) as string[];
        let successCount = 0;
        let failCount = 0;

        for (const agentId of agentIds) {
            try {
                const response = await configClient.unassignConfig({ agentId });
                if (response.success) {
                    successCount++;
                } else {
                    failCount++;
                }
            } catch {
                failCount++;
            }
        }

        notifications.show({
            title: 'Bulk Unassign Complete',
            message: `${successCount} succeeded, ${failCount} failed`,
            color: failCount > 0 ? 'yellow' : 'green',
            icon: <CheckCircledIcon />,
        });

        fetchAssignments();
        setSelectedAssignments(new Set());
        closeBulkUnassignModal();
    }, [selectedAssignments, configClient, fetchAssignments, closeBulkUnassignModal]);

    const confirmUnassign = useCallback((agentId: string) => {
        setAssignmentToUnassign(agentId);
        openUnassignModal();
    }, [openUnassignModal]);

    useEffect(() => {
        fetchAssignments();
        fetchConfigs();
    }, [fetchAssignments, fetchConfigs]);

    // Apply client-side filters for status and source
    const filteredAssignments = useMemo(() => {
        let result = assignments;

        if (statusFilter) {
            const statusValue = parseInt(statusFilter, 10);
            result = result.filter(a => a.status === statusValue);
        }

        if (sourceFilter) {
            const sourceValue = parseInt(sourceFilter, 10);
            result = result.filter(a => a.source === sourceValue);
        }

        return result;
    }, [assignments, statusFilter, sourceFilter]);

    const formatDate = (timestamp?: { seconds?: bigint; nanos?: number }) => {
        if (!timestamp?.seconds) return 'N/A';
        return new Date(Number(timestamp.seconds) * 1000).toLocaleString();
    };

    const assignmentColumns = useMemo<ColumnConfig<ConfigAssignmentInfo>[]>(() => [
        {
            key: 'agentId',
            label: 'Agent ID',
            visible: true,
            render: (value: unknown) => (
                <Group gap="xs" justify="center">
                    <Text size="sm" style={{ fontFamily: 'monospace' }}>{String(value)}</Text>
                    <Link to="/agents/$agentId" params={{ agentId: String(value) }}>
                        <ActionIcon variant="subtle" size="sm">
                            <ExternalLinkIcon />
                        </ActionIcon>
                    </Link>
                </Group>
            ),
        },
        {
            key: 'configId',
            label: 'Config',
            visible: true,
            render: (value: unknown) => (
                <Text size="sm" fw={500}>{String(value)}</Text>
            ),
        },
        {
            key: 'source',
            label: 'Source',
            visible: true,
            render: (value: unknown) => (
                <ConfigSourceBadge source={value as ConfigSource} />
            ),
        },
        {
            key: 'status',
            label: 'Status',
            visible: true,
            render: (value: unknown, row: ConfigAssignmentInfo) => (
                <ApplicationStatusBadge status={value as ConfigApplicationStatus} errorMessage={row.errorMessage} />
            ),
        },
        {
            key: 'assignedAt',
            label: 'Assigned At',
            visible: true,
            render: (value: unknown) => (
                <Text size="sm">{formatDate(value as { seconds?: bigint; nanos?: number } | undefined)}</Text>
            ),
        },
        {
            key: 'actions',
            label: '',
            visible: true,
            render: (_value: unknown, row: ConfigAssignmentInfo) => (
                <ActionIcon
                    variant="subtle"
                    color="red"
                    onClick={() => confirmUnassign(row.agentId)}
                    title="Unassign config"
                >
                    <Cross2Icon />
                </ActionIcon>
            ),
        },
    ], [confirmUnassign]);

    const statusOptions = [
        { value: String(ConfigApplicationStatus.PENDING), label: 'Pending' },
        { value: String(ConfigApplicationStatus.APPLIED), label: 'Applied' },
        { value: String(ConfigApplicationStatus.FAILED), label: 'Failed' },
    ];

    const sourceOptions = [
        { value: String(ConfigSource.DEFAULT), label: 'Default' },
        { value: String(ConfigSource.BOOTSTRAP), label: 'Bootstrap' },
        { value: String(ConfigSource.MANUAL), label: 'Manual' },
    ];

    return (
        <Stack gap="md">
            {/* Label-based assignment section */}
            <LabelSelector onAssignmentComplete={fetchAssignments} />

            {/* Filters */}
            <Group gap="md">
                <Select
                    placeholder="Filter by config"
                    data={availableConfigs.map(c => ({ value: c.id, label: c.id }))}
                    value={configFilter}
                    onChange={setConfigFilter}
                    clearable
                    style={{ minWidth: 200 }}
                />
                <Select
                    placeholder="Filter by status"
                    data={statusOptions}
                    value={statusFilter}
                    onChange={setStatusFilter}
                    clearable
                    style={{ minWidth: 150 }}
                />
                <Select
                    placeholder="Filter by source"
                    data={sourceOptions}
                    value={sourceFilter}
                    onChange={setSourceFilter}
                    clearable
                    style={{ minWidth: 150 }}
                />
                {selectedAssignments.size > 0 && (
                    <Button
                        variant="light"
                        color="red"
                        size="sm"
                        onClick={openBulkUnassignModal}
                    >
                        Unassign {selectedAssignments.size} selected
                    </Button>
                )}
            </Group>

            {/* Assignments table */}
            <Table<ConfigAssignmentInfo>
                title="Config Assignments"
                data={filteredAssignments}
                columns={assignmentColumns}
                rowKey="agentId"
                selectable
                selectedKeys={selectedAssignments}
                onSelectionChange={setSelectedAssignments}
            />

            {/* Unassign Modal */}
            <Modal opened={unassignModalOpened} onClose={closeUnassignModal} title="Unassign Config">
                <Text>Are you sure you want to remove the config assignment from this agent?</Text>
                <Text size="sm" c="dimmed" mt="xs">
                    The agent will fall back to the default configuration.
                </Text>
                <Group justify="flex-end" mt="md">
                    <Button variant="default" onClick={closeUnassignModal}>Cancel</Button>
                    <Button color="red" onClick={handleUnassign}>Unassign</Button>
                </Group>
            </Modal>

            {/* Bulk Unassign Modal */}
            <Modal opened={bulkUnassignModalOpened} onClose={closeBulkUnassignModal} title="Bulk Unassign Configs">
                <Text>
                    Are you sure you want to remove config assignments from {selectedAssignments.size} agent{selectedAssignments.size !== 1 ? 's' : ''}?
                </Text>
                <Text size="sm" c="dimmed" mt="xs">
                    These agents will fall back to the default configuration.
                </Text>
                <Group justify="flex-end" mt="md">
                    <Button variant="default" onClick={closeBulkUnassignModal}>Cancel</Button>
                    <Button color="red" onClick={handleBulkUnassign}>
                        Unassign {selectedAssignments.size} Agent{selectedAssignments.size !== 1 ? 's' : ''}
                    </Button>
                </Group>
            </Modal>
        </Stack>
    );
}
