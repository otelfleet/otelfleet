import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { HistoryTab } from './agentDetail';
import {
    mockEffectiveConfig,
    mockHistory,
    SAMPLE_EFFECTIVE_CONFIG,
} from './agentDetailMocks';

/**
 * The "History" tab: past effective configurations keyed by revision, with the
 * selected revision's config rendered in the read-only `Editor` on the right.
 *
 * Note: the Monaco editor loads its worker assets from a CDN, so this tab needs
 * network access the first time it renders in Storybook.
 */
const meta = {
    title: 'Agents/Detail/HistoryTab',
    component: HistoryTab,
    parameters: {
        layout: 'fullscreen',
    },
    decorators: [
        (Story) => (
            <div style={{ height: '95vh', padding: 16, display: 'flex', boxSizing: 'border-box' }}>
                <Story />
            </div>
        ),
    ],
} satisfies Meta<typeof HistoryTab>;

export default meta;

type Story = StoryObj<typeof meta>;

/** A handful of revisions; the newest is selected by default. */
export const Default: Story = {
    args: {
        history: mockHistory(),
    },
};

/** A single revision — the agent has only ever reported one config. */
export const SingleRevision: Story = {
    args: {
        history: mockHistory(1),
    },
};

/** Enough revisions that the revision list scrolls. */
export const ManyRevisions: Story = {
    args: {
        history: mockHistory(30),
    },
};

/** A revision that reports several config files; only the first is shown. */
export const MultipleConfigFiles: Story = {
    args: {
        history: [
            mockEffectiveConfig({
                'collector.yaml': SAMPLE_EFFECTIVE_CONFIG,
                'extensions.yaml': 'extensions:\n  health_check: {}\n',
            }),
        ],
    },
};

/** A revision recorded with no config files attached. */
export const EmptyRevision: Story = {
    args: {
        history: [mockEffectiveConfig({})],
    },
};

/** No history recorded yet. */
export const NoHistory: Story = {
    args: {
        history: [],
    },
};

/** History still loading from the API. */
export const Loading: Story = {
    args: {
        history: [],
        loading: true,
    },
};
