import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { fn } from 'storybook/test';
import { ConfigSource } from '../../gen/api/pkg/api/config/v1alpha1/config_pb';
import { ConfigAssignmentSection } from './agentDetail';
import { mockAssignment } from './agentDetailMocks';

/**
 * The "Config Assignment" panel on the agent detail page. Shows the assigned
 * config, its source, and assignment time, plus the assign/change/unassign
 * actions.
 */
const meta = {
    title: 'Agents/Detail/ConfigAssignmentSection',
    component: ConfigAssignmentSection,
    args: {
        onAssign: fn(),
        onUnassign: fn(),
    },
} satisfies Meta<typeof ConfigAssignmentSection>;

export default meta;

type Story = StoryObj<typeof meta>;

/** A manually-assigned config — shows Change + Unassign actions. */
export const Assigned: Story = {
    args: {
        assignment: mockAssignment({ configId: 'prod-traces', source: ConfigSource.MANUAL }),
    },
};

/** A config inherited from bootstrap. */
export const BootstrapSource: Story = {
    args: {
        assignment: mockAssignment({ configId: 'bootstrap-default', source: ConfigSource.BOOTSTRAP }),
    },
};

/** No config assigned — only the "Assign Config" action is shown. */
export const Unassigned: Story = {
    args: {
        assignment: null,
    },
};
