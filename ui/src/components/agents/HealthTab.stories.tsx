import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { HealthTab } from './agentDetail';
import { mockStatus, NESTED_COMPONENT_HEALTH } from './agentDetailMocks';

/**
 * The "Health" tab: an overview card plus a recursive, expandable component
 * health table.
 */
const meta = {
    title: 'Agents/Detail/HealthTab',
    component: HealthTab,
} satisfies Meta<typeof HealthTab>;

export default meta;

type Story = StoryObj<typeof meta>;

/** Healthy agent with a nested tree of component health. */
export const Healthy: Story = {
    args: {
        health: mockStatus({ componentHealthMap: NESTED_COMPONENT_HEALTH }).health,
    },
};

/** Unhealthy agent with a top-level error and failing components. */
export const Unhealthy: Story = {
    args: {
        health: mockStatus({
            healthy: false,
            statusMessage: 'Pipeline degraded',
            lastError: 'exporter "otlphttp": connection refused',
            componentHealthMap: NESTED_COMPONENT_HEALTH,
        }).health,
    },
};

/** Healthy with no per-component breakdown (overview only). */
export const NoComponents: Story = {
    args: {
        health: mockStatus().health,
    },
};

/** No health data reported. */
export const NoData: Story = {
    args: {
        health: undefined,
    },
};
