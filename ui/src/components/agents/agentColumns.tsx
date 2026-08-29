import { Button, Group, ActionIcon, Text } from '@mantine/core';
import { Link } from '@tanstack/react-router';
import { TrashIcon } from '@radix-ui/react-icons';
import type { CollectorView } from '../../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';
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
}: BuildAgentColumnsOptions): ColumnConfig<CollectorView>[] {
    return [
        {
            key: 'name',
            label: 'Name',
            visible: true,
            render: (_: unknown, row: CollectorView) => {
                return <Text fw={500}>{row.desc?.friendlyName || 'Unknown'}</Text>
            }
        },
        {
            key: 'connection',
            label: 'Connection',
            visible: true,
            render: (_: unknown, row: CollectorView) => {
                return <StatusBadge state={row.status?.connStatus?.state ?? 0} />
            }
        },
        {
            key: 'health',
            label: 'Health',
            visible: true,
            render: (_: unknown, row: CollectorView) => {
                return <HealthBadge health={row.status?.health} />
            }
        },
        {
            key: 'configStatus',
            label: 'Config Sync',
            visible: true,
            render: (_: unknown, row: CollectorView) => {
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
            render: (_: unknown, row: CollectorView) => {
                if (!row.desc?.id) return null;
                return (
                    <Group gap="xs" justify="center">
                        <Link to="/deployments/$agentId" params={{ agentId: row.desc.id }}>
                            <Button size="xs" variant="light">
                                Details
                            </Button>
                        </Link>
                        <ActionIcon
                            color="red"
                            variant="subtle"
                            size="lg"
                            onClick={() => onDelete(row.desc!.id, row.desc!.friendlyName || 'Unknown')}
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
