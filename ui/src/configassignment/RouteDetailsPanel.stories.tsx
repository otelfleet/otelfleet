import type { Meta, StoryObj } from '@storybook/tanstack-react';

import { RouteDetailsPanel } from './RouteDetailsPanel';

/**
 * Details of the route selected in the graph, including every collector that
 * lands on its collector config.
 */
const meta = {
  title: 'ConfigAssignment/RouteDetailsPanel',
  component: RouteDetailsPanel,
} satisfies Meta<typeof RouteDetailsPanel>;

export default meta;

type Story = StoryObj<typeof meta>;

export const WithCollectors: Story = {
  args: {
    route: {
      name: 'production',
      configRef: 'production-eu',
      use: '',
      matchers: ['user-defined | env == production', 'environment | host.name =~ ^eu-.*'],
      collectors: 2,
      isRoot: false,
      isDef: false,
      selected: true,
      matched: false,
    },
    collectors: ['019f520781397660938bb7fefc8bc910', '019f5207813976609ffa1120bb7fefc8'],
  },
};

export const Unassigned: Story = {
  args: {
    route: {
      name: 'root',
      configRef: '',
      use: '',
      matchers: [],
      collectors: 0,
      isRoot: true,
      isDef: false,
      selected: true,
      matched: false,
    },
    collectors: [],
  },
};
