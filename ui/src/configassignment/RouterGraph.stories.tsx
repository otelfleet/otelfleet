import type { Meta, StoryObj } from '@storybook/tanstack-react';

import { LabelType, MatchType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import { RouterGraph } from './RouterGraph';
import { emptyRouterValues } from './router';

/**
 * Read-only view of the routing tree. Each node shows its label matchers in
 * `<scope> | key == value` form and how many collectors land on its config.
 */
const meta = {
  title: 'ConfigAssignment/RouterGraph',
  component: RouterGraph,
  decorators: [
    (Story) => (
      <div style={{ height: '80vh', width: '100%' }}>
        <Story />
      </div>
    ),
  ],
  parameters: { layout: 'fullscreen' },
} satisfies Meta<typeof RouterGraph>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: { value: emptyRouterValues(), assignedConfigRefs: [] },
};

export const Tree: Story = {
  args: {
    assignedConfigRefs: ['base', 'base', 'production-eu', 'production-us', 'production-us'],
    value: {
      configRef: 'base',
      root: {
        name: 'root',
        configRef: 'base',
        use: '',
        filters: [],
        routes: [
          {
            name: 'production',
            configRef: 'production-us',
            use: '',
            filters: [
              {
                type: LabelType.LabelTypeOtelfleet,
                labels: [{ matchType: MatchType.EQ, key: 'env', value: 'production' }],
              },
            ],
            routes: [{ name: 'eu', configRef: '', use: 'eu-hosts', filters: [], routes: [] }],
          },
        ],
      },
      defs: [
        {
          name: 'eu-hosts',
          configRef: 'production-eu',
          use: '',
          filters: [
            {
              type: LabelType.LabelTypeNonIdentifying,
              labels: [{ matchType: MatchType.RE, key: 'host.name', value: '^eu-.*' }],
            },
          ],
          routes: [],
        },
      ],
    },
  },
};
