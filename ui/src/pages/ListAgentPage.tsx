import { AgentService, AgentState as AgentStateEnum, } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type { AgentDescription, AgentDescriptionAndStatus, AgentStatus, AgentState } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { useClient } from '../api';
import { useEffect, useState, useCallback } from 'react';
import { notifyGRPCError } from '../api/notifications';
import { Badge } from '@mantine/core';
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

const agentColumns: ColumnConfig<AgentDescriptionAndStatus>[] = [
    {
        key: 'agent', label: 'Name', visible: true, render: (value: AgentDescription) => {
            return <div>{value.friendlyName}</div>
        }
    },
    {
        key: 'status', label: 'Status', visible: true, render: (value: AgentStatus) => {
            return <StatusBadge state={value.state} />
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
        />
    )
}