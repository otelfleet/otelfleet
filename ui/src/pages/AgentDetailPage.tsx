import { useEffect, useState, useCallback } from 'react';
import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { AgentService } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type {
    AgentDescription,
    AgentStatus,
    EffectiveConfig,
} from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import type {
    ConfigReference,
    GetAgentConfigResponse,
} from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import {
    Text,
    Group,
    Stack,
    Loader,
    Center,
    Alert,
    Button,
    Modal,
    Select,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { AlertCircle } from 'react-feather';
import { CheckCircledIcon } from '@radix-ui/react-icons';
import { AgentDetailView } from '../components/agents/agentDetail';

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
    const [history, setHistory] = useState<EffectiveConfig[]>([]);
    const [historyLoading, setHistoryLoading] = useState(true);

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

    const fetchHistory = useCallback(async () => {
        setHistoryLoading(true);
        try {
            const response = await agentClient.agentHistory({ agentId, offset: 0n, limit: 50n });
            setHistory(response.effectiveConfig);
        } catch (err) {
            notifyGRPCError('Failed to load agent history', err);
            setHistory([]);
        } finally {
            setHistoryLoading(false);
        }
    }, [agentId, agentClient]);

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
        fetchHistory();
    }, [agentId, agentClient, fetchConfigAssignment, fetchHistory]);

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
            <AgentDetailView
                agent={agent}
                status={status}
                assignment={configAssignment}
                onAssign={openAssignModal}
                onUnassign={openUnassignModal}
                history={history}
                historyLoading={historyLoading}
            />

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
