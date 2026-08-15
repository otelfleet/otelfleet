import { Badge, Tooltip } from '@mantine/core';
import {
    AgentState as AgentStateEnum,
    ConfigSyncStatus as ConfigSyncStatusEnum,
} from '../../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type {
    AgentState,
    ComponentHealth,
    ConfigSyncStatus,
} from '../../gen/api/pkg/api/agents/v1alpha1/agents_pb';

export function StatusBadge({ state }: { state: AgentState }) {
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

export function HealthBadge({ health }: { health?: ComponentHealth }) {
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

export function ConfigSyncStatusBadge({ status, reason }: { status?: ConfigSyncStatus; reason?: string }) {
    const statusMap: Record<number, { color: string; label: string }> = {
        [ConfigSyncStatusEnum.UNKNOWN]: { color: 'gray', label: 'None' },
        [ConfigSyncStatusEnum.IN_SYNC]: { color: 'green', label: 'In Sync' },
        [ConfigSyncStatusEnum.OUT_OF_SYNC]: { color: 'yellow', label: 'Out of Sync' },
        [ConfigSyncStatusEnum.APPLYING]: { color: 'blue', label: 'Applying' },
        [ConfigSyncStatusEnum.ERROR]: { color: 'red', label: 'Error' },
    };

    const { color, label } = statusMap[status ?? 0] ?? { color: 'gray', label: 'None' };

    return (
        <Tooltip label={reason} disabled={!reason}>
            <Badge color={color} variant="filled" radius="sm">
                {label}
            </Badge>
        </Tooltip>
    );
}

