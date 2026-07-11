import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { AgentState, ConfigSyncStatus } from '../../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { AgentHeader } from './agentDetail';
import { mockAgent, mockStatus } from './agentDetailMocks';

/**
 * The header block at the top of the agent detail page: name, ID, and the
 * connection / health / config-sync badges.
 */
const meta = {
    title: 'Agents/Detail/AgentHeader',
    component: AgentHeader,
} satisfies Meta<typeof AgentHeader>;

export default meta;

type Story = StoryObj<typeof meta>;

/** Connected, healthy, and in sync. */
export const Healthy: Story = {
    args: {
        agent: mockAgent(),
        status: mockStatus(),
    },
};

/** Connected but unhealthy with a config-sync error. */
export const Unhealthy: Story = {
    args: {
        agent: mockAgent(),
        status: mockStatus({
            healthy: false,
            configSyncStatus: ConfigSyncStatus.ERROR,
        }),
    },
};

/** Disconnected agent with unknown health/sync. */
export const Disconnected: Story = {
    args: {
        agent: mockAgent(),
        status: mockStatus({
            state: AgentState.DISCONNECTED,
            hasHealth: false,
            configSyncStatus: ConfigSyncStatus.UNKNOWN,
        }),
    },
};

/** No data yet (nulls) — falls back to placeholders. */
export const NoData: Story = {
    args: {
        agent: null,
        status: null,
    },
};
