import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { DetailsTab } from './agentDetail';
import { mockAgent } from './agentDetailMocks';

/**
 * The "Details" tab: identifying / non-identifying attribute tables and the
 * capabilities card.
 */
const meta = {
    title: 'Agents/Detail/DetailsTab',
    component: DetailsTab,
} satisfies Meta<typeof DetailsTab>;

export default meta;

type Story = StoryObj<typeof meta>;

/** A fully-described agent with attributes and capabilities. */
export const Populated: Story = {
    args: {
        agent: mockAgent(),
    },
};

/** An agent that reported no attributes or capabilities. */
export const Empty: Story = {
    args: {
        agent: mockAgent({
            identifying: [],
            nonIdentifying: [],
            capabilities: [],
        }),
    },
};

/** No agent data available. */
export const NoData: Story = {
    args: {
        agent: null,
    },
};
