import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { useState } from 'react';

import { LabelType, MatchType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import { RouterForm } from './RouterForm';
import { emptyRouterValues, type RouterValues } from './router';

const COLLECTOR_CONFIGS = ['base', 'production-eu', 'production-us'];

function StatefulRouterForm({ initial }: { initial: RouterValues }) {
  const [value, setValue] = useState(initial);
  return (
    <RouterForm value={value} collectorConfigs={COLLECTOR_CONFIGS} onChange={setValue} />
  );
}

/**
 * The whole `route.v1alpha1.Router`: a default config ref, the root route tree
 * and the reusable definitions routes can `use`. Validate, preview and save
 * live on the page around it.
 */
const meta = {
  title: 'ConfigAssignment/RouterForm',
  component: StatefulRouterForm,
} satisfies Meta<typeof StatefulRouterForm>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: { initial: emptyRouterValues() },
};

export const WithDefs: Story = {
  args: {
    initial: {
      configRef: 'base',
      root: {
        name: 'root',
        configRef: '',
        use: '',
        filters: [],
        routes: [{ name: 'eu', configRef: '', use: 'eu-hosts', filters: [], routes: [] }],
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
