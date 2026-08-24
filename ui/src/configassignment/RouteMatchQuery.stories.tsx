import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { useState } from 'react';

import { LabelType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import { RouteMatchQuery, type LabelQueryRow, type RouteMatchResult } from './RouteMatchQuery';

const ROWS: LabelQueryRow[] = [
  { type: LabelType.LabelTypeNonIdentifying, key: 'os.type', value: 'linux' },
];

function StatefulRouteMatchQuery({ initial, result }: { initial: LabelQueryRow[]; result?: RouteMatchResult }) {
  const [rows, setRows] = useState(initial);
  return <RouteMatchQuery rows={rows} result={result} onChange={setRows} onMatch={() => {}} />;
}

/**
 * Label querier: describes a hypothetical collector and asks `MatchRouter`
 * which route it lands on. Collapsed to a button until opened.
 */
const meta = {
  title: 'ConfigAssignment/RouteMatchQuery',
  component: StatefulRouteMatchQuery,
} satisfies Meta<typeof StatefulRouteMatchQuery>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Collapsed: Story = {
  args: { initial: ROWS },
};

export const Matched: Story = {
  args: {
    initial: ROWS,
    result: { path: ['root', 'production', 'eu'], configRef: 'production-eu', matched: true },
  },
};

export const NoMatch: Story = {
  args: { initial: ROWS, result: { path: [], configRef: 'base', matched: false } },
};
