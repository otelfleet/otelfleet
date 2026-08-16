import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { useState } from 'react';
import { fn } from 'storybook/test';
import { create } from '@bufbuild/protobuf';
import {
    AgentDescriptionAndStatusSchema,
    AgentState,
    ConfigSyncStatus,
} from '../../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import type { AgentDescriptionAndStatus } from '../../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { Table } from '../Table';
import { buildAgentColumns } from './agentColumns';

/**
 * The agents table as rendered on the `/agents` page: the generic `Table`
 * driven by `buildAgentColumns` (shared with the live page, so this stays in
 * sync). Covers every connection / health / config-sync state
 * with mock data — no backend required.
 *
 * Selection, column show/hide (the ☰ menu), and the expandable error row are
 * all interactive here.
 */
const meta = {
    title: 'Agents/AgentsTable',
    // A thin wrapper so selection state is interactive inside Storybook.
    render: (args) => <AgentsTableDemo {...args} />,
    args: {
        selectable: true,
    },
    argTypes: {
        selectable: { control: 'boolean' },
    },
} satisfies Meta<AgentsTableDemoProps>;

export default meta;

type Story = StoryObj<typeof meta>;

// --- Mock data -------------------------------------------------------------

function mockAgent(init: {
    id: string;
    name: string;
    state: AgentState;
    healthy?: boolean;
    lastError?: string;
    syncStatus: ConfigSyncStatus;
    syncReason?: string;
}): AgentDescriptionAndStatus {
    return create(AgentDescriptionAndStatusSchema, {
        agent: {
            id: init.id,
            friendlyName: init.name,
        },
        status: {
            connStatus: { state: init.state },
            syncStatus: {
                status: init.syncStatus,
                configSyncReason: init.syncReason ?? '',
            },
            // Omit health entirely for the "Unknown" case.
            health: init.healthy === undefined
                ? undefined
                : { healthy: init.healthy, lastError: init.lastError ?? '' },
        },
    });
}

const AGENTS: AgentDescriptionAndStatus[] = [
    mockAgent({
        id: 'agent-web-01',
        name: 'web-collector-01',
        state: AgentState.CONNECTED,
        healthy: true,
        syncStatus: ConfigSyncStatus.IN_SYNC,
    }),
    mockAgent({
        id: 'agent-edge-eu',
        name: 'edge-collector-eu',
        state: AgentState.CONNECTED,
        healthy: false,
        lastError: 'exporter "otlphttp": connection refused to https://otel.example.com:4318',
        syncStatus: ConfigSyncStatus.ERROR,
        syncReason: 'Failed to apply config: invalid exporter endpoint',
    }),
    mockAgent({
        id: 'agent-batch-03',
        name: 'batch-collector-03',
        state: AgentState.CONNECTED,
        healthy: true,
        syncStatus: ConfigSyncStatus.OUT_OF_SYNC,
        syncReason: 'Assigned config newer than reported hash',
    }),
    mockAgent({
        id: 'agent-apply-04',
        name: 'ingest-collector-04',
        state: AgentState.CONNECTED,
        healthy: true,
        syncStatus: ConfigSyncStatus.APPLYING,
    }),
    mockAgent({
        id: 'agent-legacy-09',
        name: 'legacy-agent-09',
        state: AgentState.DISCONNECTED,
        // no health -> "Unknown"
        syncStatus: ConfigSyncStatus.UNKNOWN,
    }),
];

// --- Interactive wrapper ---------------------------------------------------

interface AgentsTableDemoProps {
    data: AgentDescriptionAndStatus[];
    selectable: boolean;
    onDelete: (agentId: string, agentName: string) => void;
}

function AgentsTableDemo({ data, selectable, onDelete }: AgentsTableDemoProps) {
    const [selectedKeys, setSelectedKeys] = useState<Set<string | number>>(new Set());
    const columns = buildAgentColumns({ onDelete });

    return (
        <Table<AgentDescriptionAndStatus>
            title="OpenTelemetry Collector agents"
            data={data}
            columns={columns}
            rowKey={(row) => row.agent?.id ?? ''}
            selectable={selectable}
            selectedKeys={selectedKeys}
            onSelectionChange={setSelectedKeys}
            expandedContent={(row) => {
                const error = row.status?.health?.lastError;
                if (!error) return null;
                return <span style={{ color: 'var(--mantine-color-red-6)' }}>{error}</span>;
            }}
        />
    );
}

// --- Stories ---------------------------------------------------------------

/** Full table with a mix of connection, health, and sync states. */
export const Default: Story = {
    args: {
        data: AGENTS,
        onDelete: fn(),
    },
};

/** Same data without the selection checkboxes. */
export const NonSelectable: Story = {
    args: {
        data: AGENTS,
        selectable: false,
        onDelete: fn(),
    },
};

/** A single healthy, in-sync agent. */
export const SingleAgent: Story = {
    args: {
        data: [AGENTS[0]],
        onDelete: fn(),
    },
};

/** No agents connected yet — an empty table body. */
export const Empty: Story = {
    args: {
        data: [],
        onDelete: fn(),
    },
};
