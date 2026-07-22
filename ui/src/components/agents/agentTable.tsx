import { Badge, Tooltip, Text, Group } from '@mantine/core';
import {
    AgentState as AgentStateEnum,
    ConfigSyncStatus as ConfigSyncStatusEnum,
} from '../../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type {
    AgentState,
    ComponentHealth,
    ConfigSyncStatus,
} from '../../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { ConfigApplicationStatus } from '../../gen/api/pkg/api/config/v1alpha1/config_pb';
import type { ConfigAssignmentInfo } from '../../gen/api/pkg/api/config/v1alpha1/config_pb';

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

export function AssignedConfigBadge({ assignment }: { assignment?: ConfigAssignmentInfo }) {
    if (!assignment?.configId) {
        return <Text size="sm" c="dimmed">(none)</Text>;
    }

    const statusMap: Record<number, { color: string; label: string }> = {
        [ConfigApplicationStatus.UNSPECIFIED]: { color: 'gray', label: '' },
        [ConfigApplicationStatus.PENDING]: { color: 'yellow', label: 'Pending' },
        [ConfigApplicationStatus.APPLIED]: { color: 'green', label: 'Applied' },
        [ConfigApplicationStatus.FAILED]: { color: 'red', label: 'Failed' },
    };

    const { color, label } = statusMap[assignment.status] ?? { color: 'gray', label: '' };

    return (
        <Tooltip label={assignment.errorMessage} disabled={!assignment.errorMessage}>
            <Group gap="xs" justify="center">
                <Text size="sm" fw={500}>{assignment.configId}</Text>
                {label && <Badge color={color} variant="light" size="xs">{label}</Badge>}
            </Group>
        </Tooltip>
    );
}
