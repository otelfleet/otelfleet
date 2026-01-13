import { AgentService, AgentState as AgentStateEnum, RemoteConfigStatuses } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type { AgentDescriptionAndStatus, AgentState, ComponentHealth, RemoteConfigStatus } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { useClient } from '../api';
import { useEffect, useState, useCallback } from 'react';
import { notifyGRPCError } from '../api/notifications';
import { Badge, Tooltip, Text, Button } from '@mantine/core';
import { Link } from '@tanstack/react-router';
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

function ConfigStatusBadge({ configStatus }: { configStatus?: RemoteConfigStatus }) {
    if (!configStatus) {
        return <Badge color="gray" variant="filled" radius="sm">Unset</Badge>
    }

    const statusMap: Record<number, { color: string; label: string }> = {
        [RemoteConfigStatuses.UNSET]: { color: 'gray', label: 'Unset' },
        [RemoteConfigStatuses.APPLIED]: { color: 'green', label: 'Applied' },
        [RemoteConfigStatuses.APPLYING]: { color: 'blue', label: 'Applying' },
        [RemoteConfigStatuses.FAILED]: { color: 'red', label: 'Failed' },
    };

    const { color, label } = statusMap[configStatus.status] ?? { color: 'gray', label: 'Unknown' };

    return (
        <Tooltip label={configStatus.errorMessage} disabled={!configStatus.errorMessage}>
            <Badge color={color} variant="filled" radius="sm">
                {label}
            </Badge>
        </Tooltip>
    );
}

const agentColumns: ColumnConfig<AgentDescriptionAndStatus>[] = [
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
        key: 'config',
        label: 'Config',
        visible: true,
        render: (_: unknown, row: AgentDescriptionAndStatus) => {
            return <ConfigStatusBadge configStatus={row.status?.remoteConfigStatus} />
        }
    },
    {
        key: 'actions',
        label: '',
        visible: true,
        render: (_: unknown, row: AgentDescriptionAndStatus) => {
            if (!row.agent?.id) return null;
            return (
                <Link to="/agents/$agentId" params={{ agentId: row.agent.id }}>
                    <Button size="xs" variant="light">
                        Details
                    </Button>
                </Link>
            );
        }
    },
]



export const AgentPage = () => {
    const client = useClient(AgentService);

    const [agentsState, setAgentsState] = useState<AgentDescriptionAndStatus[]>([])

    const handleListAgents = useCallback(async () => {
        try {
            const response = await client.listAgents({
                withStatus: true,
            });
            setAgentsState(response.agents)
        } catch (error) {
            notifyGRPCError("Failed to list agents", error)
        }
    }, [client])

    useEffect(() => {
        handleListAgents();
    }, [handleListAgents])

    return (
        <Table<AgentDescriptionAndStatus>
            title="OpenTelemetry Collector agents"
            data={agentsState}
            columns={agentColumns}
            rowKey={(row) => row.agent?.id ?? ''}
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
    )
}