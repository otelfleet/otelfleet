import { Button, Group, ActionIcon, Text } from '@mantine/core';
import { Link } from '@tanstack/react-router';
import { TrashIcon } from '@radix-ui/react-icons';
import type { CollectorDescriptionAndStatus } from '../../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';
import type { ColumnConfig } from '../Table';
import {
    StatusBadge,
    HealthBadge,
    ConfigSyncStatusBadge,
} from './agentTable';

export interface BuildAgentColumnsOptions {
    /** Invoked when the row's delete action is triggered. */
    onDelete: (agentId: string, agentName: string) => void;
}

/**
 * Builds the column configuration for the agents table. Shared between the
 * live agents page and Storybook so both render identical columns.
 */
export function buildAgentColumns({
    onDelete,
}: BuildAgentColumnsOptions): ColumnConfig<CollectorDescriptionAndStatus>[] {
    return [
        {
            key: 'name',
            label: 'Name',
            visible: true,
            render: (_: unknown, row: CollectorDescriptionAndStatus) => {
                return <Text fw={500}>{row.collector?.friendlyName || 'Unknown'}</Text>
            }
        },
        {
            key: 'connection',
            label: 'Connection',
            visible: true,
            render: (_: unknown, row: CollectorDescriptionAndStatus) => {
                return <StatusBadge state={row.status?.connStatus?.state ?? 0} />
            }
        },
        {
            key: 'health',
            label: 'Health',
            visible: true,
            render: (_: unknown, row: CollectorDescriptionAndStatus) => {
                return <HealthBadge health={row.status?.health} />
            }
        },
        {
            key: 'configStatus',
            label: 'Config Sync',
            visible: true,
            render: (_: unknown, row: CollectorDescriptionAndStatus) => {
                return <ConfigSyncStatusBadge
                    status={row.status?.syncStatus?.status}
                    reason={row.status?.syncStatus?.configSyncReason}
                />
            }
        },
        {
            key: 'actions',
            label: '',
            visible: true,
            render: (_: unknown, row: CollectorDescriptionAndStatus) => {
                if (!row.collector?.id) return null;
                return (
                    <Group gap="xs" justify="center">
                        <Link to="/deployments/$agentId" params={{ agentId: row.collector.id }}>
                            <Button size="xs" variant="light">
                                Details
                            </Button>
                        </Link>
                        <ActionIcon
                            color="red"
                            variant="subtle"
                            size="lg"
                            onClick={() => onDelete(row.collector!.id, row.collector!.friendlyName || 'Unknown')}
                            title="Delete agent"
                        >
                            <TrashIcon width={18} height={18} />
                        </ActionIcon>
                    </Group>
                );
            }
        },
    ];
}
