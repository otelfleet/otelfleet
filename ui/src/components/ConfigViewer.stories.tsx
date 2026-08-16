import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { ConfigViewer } from './ConfigViewer';
import { SAMPLE_EFFECTIVE_CONFIG } from './agents/agentDetailMocks';

const meta = {
    title: 'Components/ConfigViewer',
    component: ConfigViewer,
    parameters: {
        layout: 'fullscreen',
    },
    decorators: [
        (Story) => (
            <div style={{ height: '95vh', padding: 16, boxSizing: 'border-box' }}>
                <Story />
            </div>
        ),
    ],
} satisfies Meta<typeof ConfigViewer>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
    args: {
        config: SAMPLE_EFFECTIVE_CONFIG,
    },
};

export const Empty: Story = {
    args: {
        config: '',
    },
};
