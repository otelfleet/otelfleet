import type { Meta, StoryObj } from '@storybook/tanstack-react';

import { RouterGraphPage } from './RouterGraphPage';

/**
 * Default config assignment view: the routing tree of the router stored under
 * the "global" key, with an Edit button into the route editor.
 *
 * Note: the router and the collector assignments come from the backend. Without
 * one running the graph is empty and gRPC error notifications appear.
 */
const meta = {
  title: 'ConfigAssignment/RouterGraphPage',
  component: RouterGraphPage,
  decorators: [
    (Story) => (
      <div style={{ height: '90vh', padding: 16, boxSizing: 'border-box' }}>
        <Story />
      </div>
    ),
  ],
  parameters: { layout: 'fullscreen' },
} satisfies Meta<typeof RouterGraphPage>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
