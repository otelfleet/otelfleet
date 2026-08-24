import { CollectorService } from '../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';
import type { CollectorDescriptionAndStatus } from '../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';
import { useClient } from '../api';
import { useEffect, useState, useCallback, useMemo } from 'react';
import { notifyGRPCError } from '../api/notifications';
import { Text, Button, Group, Modal, Stack } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { CheckCircledIcon } from '@radix-ui/react-icons';
import { Table } from '../components/Table'
import { buildAgentColumns } from '../components/agents/agentColumns'

export const AgentPage = () => {
    const agentClient = useClient(CollectorService);

    const [agentsState, setAgentsState] = useState<CollectorDescriptionAndStatus[]>([]);
    const [deleteModalOpened, { open: openDeleteModal, close: closeDeleteModal }] = useDisclosure(false);
    const [agentToDelete, setAgentToDelete] = useState<{ id: string; name: string } | null>(null);
    const [deleting, setDeleting] = useState(false);

    const handleListAgents = useCallback(async () => {
        try {
            const response = await agentClient.listCollectors({
                withStatus: true,
            });
            setAgentsState(response.collectors);
        } catch (error) {
            notifyGRPCError("Failed to list agents", error);
        }
    }, [agentClient]);

    const handleDeleteAgent = useCallback(async () => {
        if (!agentToDelete) return;
        setDeleting(true);
        try {
            await agentClient.deleteCollector({ collectorId: agentToDelete.id });
            notifications.show({
                title: 'Agent Deleted',
                message: `Agent "${agentToDelete.name}" has been deleted`,
                color: 'green',
                icon: <CheckCircledIcon />,
            });
            handleListAgents();
        } catch (error) {
            notifyGRPCError("Failed to delete agent", error);
        } finally {
            setDeleting(false);
            closeDeleteModal();
            setAgentToDelete(null);
        }
    }, [agentClient, agentToDelete, handleListAgents, closeDeleteModal]);

    const confirmDelete = useCallback((agentId: string, agentName: string) => {
        setAgentToDelete({ id: agentId, name: agentName });
        openDeleteModal();
    }, [openDeleteModal]);

    useEffect(() => {
        handleListAgents();
    }, [handleListAgents]);

    const agentColumns = useMemo(
        () => buildAgentColumns({ onDelete: confirmDelete }),
        [confirmDelete]
    );

    return (
        <>
            <Table<CollectorDescriptionAndStatus>
                title="OpenTelemetry Collector agents"
                data={agentsState}
                columns={agentColumns}
                rowKey={(row) => row.collector?.id ?? ''}
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