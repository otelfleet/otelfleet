import {
  Table,
  type ColumnConfig
} from '../components/Table'

import { useClient } from '../api';
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import { AgentService } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type { AgentDescriptionAndStatus } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { useState, useEffect, useCallback, useMemo } from 'react';
import type { ConfigReference, ConfigAssignmentInfo } from '../gen/api/pkg/api/config/v1alpha1/config_pb'
import { Link } from '@tanstack/react-router'
import { Group, Button, ActionIcon, Modal, Text, Badge, Stack, Checkbox, ScrollArea, Box, TextInput } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { notifyGRPCError } from '../api/notifications';
import { CheckCircledIcon, TrashIcon, Pencil1Icon, PersonIcon, MagnifyingGlassIcon } from '@radix-ui/react-icons';

// Extended type to include assignment count
type ConfigWithCount = ConfigReference & { assignmentCount: number };

export const ConfigPage = () => {
  const configClient = useClient(ConfigService);
  const agentClient = useClient(AgentService);

  const [configState, setConfigState] = useState<ConfigWithCount[]>([]);
  const [allAssignments, setAllAssignments] = useState<ConfigAssignmentInfo[]>([]);
  const [agents, setAgents] = useState<AgentDescriptionAndStatus[]>([]);
  const [deleteModalOpened, { open: openDeleteModal, close: closeDeleteModal }] = useDisclosure(false);
  const [assignModalOpened, { open: openAssignModal, close: closeAssignModal }] = useDisclosure(false);
  const [configToDelete, setConfigToDelete] = useState<string | null>(null);
  const [configToAssign, setConfigToAssign] = useState<string | null>(null);
  const [selectedAgentIds, setSelectedAgentIds] = useState<Set<string>>(new Set());
  const [agentSearchFilter, setAgentSearchFilter] = useState('');
  const [assigning, setAssigning] = useState(false);

  const fetchAssignments = useCallback(async () => {
    try {
      const response = await configClient.listConfigAssignments({});
      setAllAssignments(response.assignments);
    } catch (error) {
      notifyGRPCError("Failed to load config assignments", error);
    }
  }, [configClient]);

  const fetchAgents = useCallback(async () => {
    try {
      const response = await agentClient.listAgents({ withStatus: false });
      setAgents(response.agents);
    } catch (error) {
      notifyGRPCError("Failed to load agents", error);
    }
  }, [agentClient]);

  const handleListConfigs = useCallback(async () => {
    try {
      const response = await configClient.listConfigs({});
      // Assignment counts will be calculated from allAssignments
      setConfigState(response.configs.map(c => ({ ...c, assignmentCount: 0 })));
    } catch (error) {
      notifyGRPCError("Failed to list configs", error);
    }
  }, [configClient]);

  // Calculate assignment counts from assignments
  const configsWithCounts = useMemo<ConfigWithCount[]>(() => {
    const countMap = new Map<string, number>();
    for (const a of allAssignments) {
      countMap.set(a.configId, (countMap.get(a.configId) ?? 0) + 1);
    }
    return configState.map(c => ({
      ...c,
      assignmentCount: countMap.get(c.id) ?? 0,
    }));
  }, [configState, allAssignments]);

  const handleDeleteConfig = useCallback(async () => {
    if (!configToDelete) return;
    try {
      await configClient.deleteConfig({ id: configToDelete });
      notifications.show({
        title: "Config deleted",
        message: "Config has been removed",
        icon: <CheckCircledIcon />,
      });
      handleListConfigs();
      fetchAssignments();
    } catch (error) {
      notifyGRPCError("Failed to delete config", error);
    } finally {
      closeDeleteModal();
      setConfigToDelete(null);
    }
  }, [configClient, handleListConfigs, fetchAssignments, configToDelete, closeDeleteModal]);

  const handleBatchAssign = useCallback(async () => {
    if (!configToAssign || selectedAgentIds.size === 0) return;
    setAssigning(true);
    try {
      const agentIds = Array.from(selectedAgentIds);
      const response = await configClient.batchAssignConfig({ agentIds, configId: configToAssign });
      notifications.show({
        title: 'Batch Assignment Complete',
        message: `${response.successful} succeeded, ${response.failed} failed`,
        color: response.failed > 0 ? 'yellow' : 'green',
        icon: <CheckCircledIcon />,
      });
      fetchAssignments();
    } catch (error) {
      notifyGRPCError("Failed to batch assign config", error);
    } finally {
      setAssigning(false);
      closeAssignModal();
      setConfigToAssign(null);
      setSelectedAgentIds(new Set());
    }
  }, [configClient, configToAssign, selectedAgentIds, fetchAssignments, closeAssignModal]);

  const confirmDelete = useCallback((configId: string) => {
    setConfigToDelete(configId);
    openDeleteModal();
  }, [openDeleteModal]);

  const openAssignModalForConfig = useCallback((configId: string) => {
    setConfigToAssign(configId);
    setSelectedAgentIds(new Set());
    setAgentSearchFilter('');
    openAssignModal();
  }, [openAssignModal]);

  // Filter agents based on search
  const filteredAgents = useMemo(() => {
    if (!agentSearchFilter) return agents;
    const lower = agentSearchFilter.toLowerCase();
    return agents.filter(a =>
      a.agent?.friendlyName?.toLowerCase().includes(lower) ||
      a.agent?.id?.toLowerCase().includes(lower)
    );
  }, [agents, agentSearchFilter]);

  // Get agents already assigned to this config
  const agentsWithThisConfig = useMemo(() => {
    if (!configToAssign) return new Set<string>();
    return new Set(
      allAssignments
        .filter(a => a.configId === configToAssign)
        .map(a => a.agentId)
    );
  }, [allAssignments, configToAssign]);

  const toggleAgentSelection = useCallback((agentId: string) => {
    setSelectedAgentIds(prev => {
      const next = new Set(prev);
      if (next.has(agentId)) {
        next.delete(agentId);
      } else {
        next.add(agentId);
      }
      return next;
    });
  }, []);

  const configColumns = useMemo<ColumnConfig<ConfigWithCount>[]>(() => [
    { key: 'id', label: 'Name', visible: true },
    {
      key: 'assignmentCount',
      label: 'Assigned Agents',
      visible: true,
      render: (value: unknown) => {
        const count = value as number;
        return (
          <Badge color={count > 0 ? 'blue' : 'gray'} variant="light">
            {count} agent{count !== 1 ? 's' : ''}
          </Badge>
        );
      }
    },
    {
      key: 'actions',
      label: 'Actions',
      visible: true,
      render: (_value: unknown, row: ConfigWithCount) => (
        <Group gap="xs" justify="center">
          <ActionIcon variant="subtle" size="lg" onClick={() => openAssignModalForConfig(row.id)} title="Assign to agents">
            <PersonIcon width={18} height={18} />
          </ActionIcon>
          <Link to="/editor" search={{ id: row.id }}>
            <ActionIcon variant="subtle" size="lg" title="Edit config">
              <Pencil1Icon width={18} height={18} />
            </ActionIcon>
          </Link>
          <ActionIcon color="red" variant="subtle" size="lg" onClick={() => confirmDelete(row.id)} title="Delete config">
            <TrashIcon width={18} height={18} />
          </ActionIcon>
        </Group>
      )
    },
  ], [confirmDelete, openAssignModalForConfig]);

  useEffect(() => {
    handleListConfigs();
    fetchAssignments();
  }, [handleListConfigs, fetchAssignments]);

  useEffect(() => {
    if (assignModalOpened) {
      fetchAgents();
    }
  }, [assignModalOpened, fetchAgents]);

  return (
    <>
      <Group style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
        <Link to="/editor" style={{ display: 'inline-block' }}>
          <Button>
            New Config
          </Button>
        </Link>
      </Group>
      <Table<ConfigWithCount>
        title="OpenTelemetry Collector configurations"
        data={configsWithCounts}
        columns={configColumns}
        rowKey="id"
      />

      {/* Delete Config Modal */}
      <Modal opened={deleteModalOpened} onClose={closeDeleteModal} title="Confirm Delete">
        <Text>Are you sure you want to delete this config? This action cannot be undone.</Text>
        <Group justify="flex-end" mt="md">
          <Button variant="default" onClick={closeDeleteModal}>Cancel</Button>
          <Button color="red" onClick={handleDeleteConfig}>Delete</Button>
        </Group>
      </Modal>

      {/* Batch Assign Modal */}
      <Modal opened={assignModalOpened} onClose={closeAssignModal} title={`Assign "${configToAssign}" to Agents`} size="lg">
        <Stack gap="md">
          <TextInput
            placeholder="Search agents..."
            leftSection={<MagnifyingGlassIcon />}
            value={agentSearchFilter}
            onChange={(e) => setAgentSearchFilter(e.currentTarget.value)}
          />
          <Text size="sm" c="dimmed">
            Select agents to assign this config to. Agents already using this config are marked.
          </Text>
          <ScrollArea h={300}>
            <Stack gap="xs">
              {filteredAgents.map((agent) => {
                const agentId = agent.agent?.id ?? '';
                const isAlreadyAssigned = agentsWithThisConfig.has(agentId);
                const isSelected = selectedAgentIds.has(agentId);
                return (
                  <Box
                    key={agentId}
                    p="xs"
                    style={{
                      borderRadius: 4,
                      backgroundColor: isSelected ? 'var(--mantine-color-blue-light)' : undefined,
                    }}
                  >
                    <Group justify="space-between">
                      <Group gap="sm">
                        <Checkbox
                          checked={isSelected}
                          onChange={() => toggleAgentSelection(agentId)}
                          disabled={isAlreadyAssigned}
                        />
                        <div>
                          <Text size="sm" fw={500}>{agent.agent?.friendlyName || 'Unknown'}</Text>
                          <Text size="xs" c="dimmed">{agentId}</Text>
                        </div>
                      </Group>
                      {isAlreadyAssigned && (
                        <Badge color="green" variant="light" size="sm">Already assigned</Badge>
                      )}
                    </Group>
                  </Box>
                );
              })}
              {filteredAgents.length === 0 && (
                <Text size="sm" c="dimmed" ta="center" py="md">
                  {agentSearchFilter ? 'No agents match your search' : 'No agents available'}
                </Text>
              )}
            </Stack>
          </ScrollArea>
          <Group justify="space-between">
            <Text size="sm" c="dimmed">
              {selectedAgentIds.size} agent{selectedAgentIds.size !== 1 ? 's' : ''} selected
            </Text>
            <Group gap="xs">
              <Button variant="default" onClick={closeAssignModal}>Cancel</Button>
              <Button onClick={handleBatchAssign} loading={assigning} disabled={selectedAgentIds.size === 0}>
                Assign to {selectedAgentIds.size} Agent{selectedAgentIds.size !== 1 ? 's' : ''}
              </Button>
            </Group>
          </Group>
        </Stack>
      </Modal>
    </>
  );
}
