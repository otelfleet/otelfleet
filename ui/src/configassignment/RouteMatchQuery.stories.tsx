import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { useState } from 'react';

import { LabelType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import { RouteMatchQuery, type LabelQueryRow } from './RouteMatchQuery';

const ROWS: LabelQueryRow[] = [
  { type: LabelType.LabelTypeNonIdentifying, key: 'os.type', value: 'linux' },
  { type: LabelType.LabelTypeIdentifying, key: 'service.name', value: 'otelcol-contrib' },
];

function StatefulRouteMatchQuery({ initial }: { initial: LabelQueryRow[] }) {
  const [rows, setRows] = useState(initial);
  return <RouteMatchQuery rows={rows} onChange={setRows} />;
}

/**
 * Label querier: describes a hypothetical collector and asks `MatchRouter`
 * which route it lands on.
 */
const meta = {
  title: 'ConfigAssignment/RouteMatchQuery',
  component: StatefulRouteMatchQuery,
} satisfies Meta<typeof StatefulRouteMatchQuery>;

export default meta;

type Story = StoryObj<typeof meta>;

export const WithFilters: Story = {
  args: { initial: ROWS },
};

export const Empty: Story = {
  args: { initial: [] },
};

