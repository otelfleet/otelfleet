import { useState, useCallback, useEffect } from 'react';
import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import type { ConfigReference } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import { AgentService } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type { AgentDescriptionAndStatus } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import {
    Paper,
    Title,
    Text,
    Group,
    Stack,
    Button,
    TextInput,
    Select,
    Badge,
    ActionIcon,
    Alert,
    Collapse,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { Cross2Icon, PlusIcon, CheckCircledIcon, ChevronDownIcon, ChevronUpIcon } from '@radix-ui/react-icons';

interface LabelPair {
    key: string;
    value: string;
}

interface LabelSelectorProps {
    onAssignmentComplete?: () => void;
}

export function LabelSelector({ onAssignmentComplete }: LabelSelectorProps) {
    const configClient = useClient(ConfigService);
    const agentClient = useClient(AgentService);

    const [labels, setLabels] = useState<LabelPair[]>([]);
    const [newLabelKey, setNewLabelKey] = useState('');
    const [newLabelValue, setNewLabelValue] = useState('');
    const [availableConfigs, setAvailableConfigs] = useState<ConfigReference[]>([]);
    const [selectedConfig, setSelectedConfig] = useState<string | null>(null);
    const [agents, setAgents] = useState<AgentDescriptionAndStatus[]>([]);
    const [matchedAgents, setMatchedAgents] = useState<AgentDescriptionAndStatus[]>([]);
    const [assigning, setAssigning] = useState(false);
    const [previewOpened, { toggle: togglePreview }] = useDisclosure(false);

    const fetchConfigs = useCallback(async () => {
        try {
            const response = await configClient.listConfigs({});
            setAvailableConfigs(response.configs);
        } catch (error) {
            notifyGRPCError('Failed to load configs', error);
        }
    }, [configClient]);

    const fetchAgents = useCallback(async () => {
        try {
            const response = await agentClient.listAgents({ withStatus: false });
            setAgents(response.agents);
        } catch (error) {
            notifyGRPCError('Failed to load agents', error);
        }
    }, [agentClient]);

    useEffect(() => {
        fetchConfigs();
        fetchAgents();
    }, [fetchConfigs, fetchAgents]);

    // Match agents based on labels
    useEffect(() => {
        if (labels.length === 0) {
            setMatchedAgents([]);
            return;
        }

        const matched = agents.filter(agent => {
            // Get agent's identifying attributes as key-value pairs
            const attrs = new Map<string, string>();
            for (const attr of agent.agent?.identifyingAttributes ?? []) {
                if (attr.value?.value.case === 'stringValue') {
                    attrs.set(attr.key, attr.value.value.value);
                }
            }
            // Also check non-identifying attributes
            for (const attr of agent.agent?.nonIdentifyingAttributes ?? []) {
                if (attr.value?.value.case === 'stringValue') {
                    attrs.set(attr.key, attr.value.value.value);
                }
            }

            // Check if all labels match
            return labels.every(label => attrs.get(label.key) === label.value);
        });

        setMatchedAgents(matched);
    }, [labels, agents]);

    const handleAddLabel = useCallback(() => {
        if (!newLabelKey.trim() || !newLabelValue.trim()) return;
        // Check for duplicate keys
        if (labels.some(l => l.key === newLabelKey.trim())) {
            notifications.show({
                title: 'Duplicate label key',
                message: 'A label with this key already exists',
                color: 'yellow',
            });
            return;
        }
        setLabels(prev => [...prev, { key: newLabelKey.trim(), value: newLabelValue.trim() }]);
        setNewLabelKey('');
        setNewLabelValue('');
    }, [newLabelKey, newLabelValue, labels]);

    const handleRemoveLabel = useCallback((index: number) => {
        setLabels(prev => prev.filter((_, i) => i !== index));
    }, []);

    const handleAssignByLabels = useCallback(async () => {
        if (!selectedConfig || labels.length === 0) return;
        setAssigning(true);
        try {
            const labelMap: { [key: string]: string } = {};
            for (const label of labels) {
                labelMap[label.key] = label.value;
            }
            const response = await configClient.assignConfigByLabels({
                labels: labelMap,
                configId: selectedConfig,
            });
            notifications.show({
                title: 'Label-Based Assignment Complete',
                message: `Assigned to ${response.successful} agents (${response.failed} failed)`,
                color: response.failed > 0 ? 'yellow' : 'green',
                icon: <CheckCircledIcon />,
            });
            // Reset form
            setLabels([]);
            setSelectedConfig(null);
            onAssignmentComplete?.();
        } catch (error) {
            notifyGRPCError('Failed to assign config by labels', error);
        } finally {
            setAssigning(false);
        }
    }, [selectedConfig, labels, configClient, onAssignmentComplete]);

    return (
        <Paper p="md" withBorder>
            <Stack gap="md">
                <Title order={4}>Assign Config by Labels</Title>
                <Text size="sm" c="dimmed">
                    Assign a configuration to all agents matching the specified labels.
                </Text>

                {/* Labels section */}
                <Stack gap="xs">
                    <Text size="sm" fw={500}>Labels</Text>
                    {labels.map((label, index) => (
                        <Group key={index} gap="xs">
                            <Badge variant="light" size="lg" style={{ flex: 1 }}>
                                {label.key} = {label.value}
                            </Badge>
                            <ActionIcon
                                variant="subtle"
                                color="red"
                                onClick={() => handleRemoveLabel(index)}
                            >
                                <Cross2Icon />
                            </ActionIcon>
                        </Group>
                    ))}
                    <Group gap="xs">
                        <TextInput
                            placeholder="Key"
                            value={newLabelKey}
                            onChange={(e) => setNewLabelKey(e.currentTarget.value)}
                            style={{ flex: 1 }}
                            size="sm"
                        />
                        <TextInput
                            placeholder="Value"
                            value={newLabelValue}
                            onChange={(e) => setNewLabelValue(e.currentTarget.value)}
                            style={{ flex: 1 }}
                            size="sm"
                        />
                        <ActionIcon
                            variant="light"
                            onClick={handleAddLabel}
                            disabled={!newLabelKey.trim() || !newLabelValue.trim()}
                        >
                            <PlusIcon />
                        </ActionIcon>
                    </Group>
                </Stack>

                {/* Preview */}
                {labels.length > 0 && (
                    <Alert
                        color={matchedAgents.length > 0 ? 'blue' : 'yellow'}
                        title={
                            <Group justify="space-between" style={{ cursor: 'pointer' }} onClick={togglePreview}>
                                <Text size="sm" fw={500}>
                                    {matchedAgents.length} agent{matchedAgents.length !== 1 ? 's' : ''} match
                                </Text>
                                {matchedAgents.length > 0 && (
                                    previewOpened ? <ChevronUpIcon /> : <ChevronDownIcon />
                                )}
                            </Group>
                        }
                    >
                        <Collapse in={previewOpened && matchedAgents.length > 0}>
                            <Stack gap="xs" mt="xs">
                                {matchedAgents.slice(0, 10).map(agent => (
                                    <Text key={agent.agent?.id} size="sm">
                                        {agent.agent?.friendlyName || agent.agent?.id}
                                    </Text>
                                ))}
                                {matchedAgents.length > 10 && (
                                    <Text size="sm" c="dimmed">
                                        ...and {matchedAgents.length - 10} more
                                    </Text>
                                )}
                            </Stack>
                        </Collapse>
                    </Alert>
                )}

                {/* Config selection */}
                <Select
                    label="Config"
                    placeholder="Select a configuration"
                    data={availableConfigs.map(c => ({ value: c.id, label: c.id }))}
                    value={selectedConfig}
                    onChange={setSelectedConfig}
                    searchable
                />

                {/* Action button */}
                <Button
                    onClick={handleAssignByLabels}
                    loading={assigning}
                    disabled={!selectedConfig || labels.length === 0 || matchedAgents.length === 0}
                >
                    Assign to {matchedAgents.length} Agent{matchedAgents.length !== 1 ? 's' : ''}
                </Button>
            </Stack>
        </Paper>
    );
}
