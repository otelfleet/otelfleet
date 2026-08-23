import type { Meta, StoryObj } from '@storybook/tanstack-react';

import { RouterPreview } from './RouterPreview';
import { diffAssignments } from './router';

/**
 * Side-by-side of the assignment the fleet runs today and the one the edited
 * router produces, from `PreviewRouterResponse.old` / `.new`.
 */
const meta = {
  title: 'ConfigAssignment/RouterPreview',
  component: RouterPreview,
} satisfies Meta<typeof RouterPreview>;

export default meta;

type Story = StoryObj<typeof meta>;

export const NoCollectors: Story = {
  args: { changes: [] },
};

export const Loading: Story = {
  args: { changes: [], loading: true },
};

export const Invalid: Story = {
  args: { changes: [], error: 'route eu references a def that doesn\'t exist: eu-hosts' },
};

export const MixedChanges: Story = {
  args: {
    changes: diffAssignments(
      { 'collector-a': 'base', 'collector-b': 'base', 'collector-c': 'production-us' },
      { 'collector-a': 'base', 'collector-b': 'production-eu', 'collector-d': 'base' },
    ),
  },
};
