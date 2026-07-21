import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { EffectiveConfigTab } from './agentDetail';
import { mockStatus, SAMPLE_EFFECTIVE_CONFIG } from './agentDetailMocks';

/**
 * The "Effective Config" tab: the agent's reported effective configuration
 * rendered in the read-only config `Editor`.
 *
 * Note: the Monaco editor loads its worker assets from a CDN, so this tab needs
 * network access the first time it renders in Storybook.
 */
const meta = {
    title: 'Agents/Detail/EffectiveConfigTab',
    component: EffectiveConfigTab,
    parameters: {
        layout: 'fullscreen',
    },
    decorators: [
        (Story) => (
            <div style={{ height: '95vh', padding: 16, display: 'flex', boxSizing: 'border-box' }}>
                <Story />
            </div>
        ),
    ],
} satisfies Meta<typeof EffectiveConfigTab>;

export default meta;

type Story = StoryObj<typeof meta>;

/** An agent reporting an effective collector config. */
export const WithConfig: Story = {
    args: {
        status: mockStatus({ effectiveConfigYaml: SAMPLE_EFFECTIVE_CONFIG }),
    },
};

/** No effective configuration reported yet. */
export const NoConfig: Story = {
    args: {
        status: mockStatus(),
    },
};
