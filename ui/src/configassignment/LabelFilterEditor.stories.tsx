import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { useState } from 'react';

import { LabelType, MatchType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import { LabelFilterEditor } from './LabelFilterEditor';
import { emptyLabelFilterValues, type LabelFilterValues } from './router';

function StatefulLabelFilterEditor({ initial }: { initial: LabelFilterValues }) {
  const [value, setValue] = useState(initial);
  return <LabelFilterEditor value={value} onChange={setValue} onRemove={() => {}} />;
}

/**
 * One `common.v1alpha1.LabelFilter` on a route: a label scope plus the key /
 * operator / value rows matched against a collector's labels.
 */
const meta = {
  title: 'ConfigAssignment/LabelFilterEditor',
  component: StatefulLabelFilterEditor,
} satisfies Meta<typeof StatefulLabelFilterEditor>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: { initial: emptyLabelFilterValues() },
};

export const HostLabels: Story = {
  args: {
    initial: {
      type: LabelType.LabelTypeNonIdentifying,
      labels: [
        { matchType: MatchType.EQ, key: 'os.type', value: 'linux' },
        { matchType: MatchType.RE, key: 'host.name', value: '^prod-.*' },
      ],
    },
  },
};
