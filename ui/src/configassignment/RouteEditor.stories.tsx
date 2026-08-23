import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { useState } from 'react';

import { LabelType, MatchType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import { RouteEditor } from './RouteEditor';
import { emptyRouteValues, type RouteValues } from './router';

const COLLECTOR_CONFIGS = ['base', 'production-eu', 'production-us'];

function StatefulRouteEditor({ initial }: { initial: RouteValues }) {
  const [value, setValue] = useState(initial);
  return (
    <RouteEditor
      isRoot
      value={value}
      collectorConfigs={COLLECTOR_CONFIGS}
      defNames={['eu-hosts']}
      onChange={setValue}
    />
  );
}

/**
 * A single `route.v1alpha1.Route` and, recursively, its nested routes. A route
 * that sets `use` inherits everything from the named definition, so its own
 * filters and children are hidden.
 */
const meta = {
  title: 'ConfigAssignment/RouteEditor',
  component: StatefulRouteEditor,
} satisfies Meta<typeof StatefulRouteEditor>;

export default meta;

type Story = StoryObj<typeof meta>;

export const EmptyRoot: Story = {
  args: { initial: emptyRouteValues('root') },
};

export const NestedRoutes: Story = {
  args: {
    initial: {
      name: 'root',
      configRef: 'base',
      use: '',
      filters: [],
      routes: [
        {
          name: 'production',
          configRef: 'production-eu',
          use: '',
          filters: [
            {
              type: LabelType.LabelTypeOtelfleet,
              labels: [{ matchType: MatchType.EQ, key: 'env', value: 'production' }],
            },
          ],
          routes: [{ ...emptyRouteValues('eu'), use: 'eu-hosts' }],
        },
      ],
    },
  },
};
