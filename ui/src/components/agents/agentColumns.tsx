import { Button, Group, ActionIcon, Text } from '@mantine/core';
import { Link } from '@tanstack/react-router';
import { TrashIcon } from '@radix-ui/react-icons';
import type { AgentDescriptionAndStatus } from '../../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type { ConfigAssignmentInfo } from '../../gen/api/pkg/api/config/v1alpha1/config_pb';
import type { ColumnConfig } from '../Table';
import {
    StatusBadge,
    HealthBadge,
    ConfigSyncStatusBadge,
    AssignedConfigBadge,
} from './agentTable';

export interface BuildAgentColumnsOptions {
    /** Config assignment info keyed by agent id, used for the "Assigned Config" column. */
    assignments: Map<string, ConfigAssignmentInfo>;
    /** Invoked when the row's delete action is triggered. */
    onDelete: (agentId: string, agentName: string) => void;
}

/**
 * Builds the column configuration for the agents table. Shared between the
 * live agents page and Storybook so both render identical columns.
 */
export function buildAgentColumns({
    assignments,
    onDelete,
}: BuildAgentColumnsOptions): ColumnConfig<AgentDescriptionAndStatus>[] {
    return [
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
                        <Link to="/deployments/$agentId" params={{ agentId: row.agent.id }}>
                            <Button size="xs" variant="light">
                                Details
                            </Button>
                        </Link>
                        <ActionIcon
                            color="red"
                            variant="subtle"
                            size="lg"
                            onClick={() => onDelete(row.agent!.id, row.agent!.friendlyName || 'Unknown')}
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
