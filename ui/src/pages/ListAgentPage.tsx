import { AgentService, AgentState as AgentStateEnum, } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type { AgentDescription, AgentDescriptionAndStatus, AgentStatus, AgentState } from '../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { useClient } from '../api';
import { useEffect, useState } from 'react';
import { notifyGRPCError } from '../api/notifications';
import { Group, Button } from '@mantine/core'
import { Badge } from '@mantine/core';

import {
    Table,
    type ColumnConfig
} from '../components/Table'
import { notifications } from '@mantine/notifications';

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
// TODO : this is bugged because we re-use the same key. key needs to handle nested pairs
    // {
    //     key: 'agent', label: 'ID', visible: true, render: (value: AgentDescription, _: AgentDescriptionAndStatus) => {
    //         return <div>
    //             {value.id}
    //         </div>
    //     }
    // },
    {
        key: 'agent', label: 'Name', visible: true, render: (value: AgentDescription, _: AgentDescriptionAndStatus) => {
            return <div>
                {value.friendlyName}
            </div>
        }
    },
    {
        key: 'status', label: 'Status', visible: true, render: (value: AgentStatus, _: AgentDescriptionAndStatus) => {
            return StatusBadge(value)
        }
    },
]



export const AgentPage = () => {
    const client = useClient(AgentService);

    const [agentsState, setAgentsState] = useState<AgentDescriptionAndStatus[]>([])

    const handleListAgents = async () => {
        try {
            console.log("starting list agents request")
            const response = await client.listAgents({
                withStatus: true,
            });
            console.log("finished list agents request")
            setAgentsState(response.agents)
            notifications.show({
                title : "yo",
                message: "got your agents big dawg"
            })
        } catch (error) {
            notifyGRPCError("Failed to list agents", error)
        }
    }

    useEffect(() => {
        handleListAgents();
    }, [])

    return (
        <>
            <Group style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
            </Group>
            <Table<AgentDescriptionAndStatus> title="OpenTelemetry Collector agents" data={agentsState} columns={agentColumns} />
        </>
    )
}