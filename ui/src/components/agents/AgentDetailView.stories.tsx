import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { AgentState, ConfigSyncStatus } from '../../gen/api/pkg/api/agents/v1alpha1/agents_pb';
import { AgentDetailView } from './agentDetail';
import {
    mockAgent,
    mockStatus,
    NESTED_COMPONENT_HEALTH,
    SAMPLE_EFFECTIVE_CONFIG,
} from './agentDetailMocks';

/**
 * The full agent detail page view — header and the Health / Details /
 * Effective Config tabs assembled together, exactly as rendered on
 * `/agents/$agentId` (minus the data fetching, which lives in the page
 * container).
 *
 * The Effective Config tab uses the Monaco editor, which loads worker assets
 * from a CDN — that tab needs network access the first time it renders.
 */
const meta = {
    title: 'Agents/Detail/AgentDetailView',
    component: AgentDetailView,
    parameters: {
        layout: 'fullscreen',
    },
    // Give the flex/height:100% layout a bounded canvas, like the app shell.
    decorators: [
        (Story) => (
            <div style={{ height: '95vh', padding: 16, boxSizing: 'border-box' }}>
                <Story />
            </div>
        ),
    ],
} satisfies Meta<typeof AgentDetailView>;

export default meta;

type Story = StoryObj<typeof meta>;

/** A connected, healthy agent with full component health. */
export const Healthy: Story = {
    args: {
        agent: mockAgent(),
        status: mockStatus({
            componentHealthMap: NESTED_COMPONENT_HEALTH,
            effectiveConfigYaml: SAMPLE_EFFECTIVE_CONFIG,
        }),
    },
};

/** A connected but unhealthy agent whose config failed to sync. */
export const Unhealthy: Story = {
    args: {
        agent: mockAgent(),
        status: mockStatus({
            healthy: false,
            statusMessage: 'Pipeline degraded',
            lastError: 'exporter "otlphttp": connection refused',
            configSyncStatus: ConfigSyncStatus.ERROR,
            componentHealthMap: NESTED_COMPONENT_HEALTH,
            effectiveConfigYaml: SAMPLE_EFFECTIVE_CONFIG,
        }),
    },
};

/** A disconnected agent with no health data. */
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
