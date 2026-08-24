import type { Meta, StoryObj } from '@storybook/tanstack-react';

import { ConfigAssignmentPage } from './ConfigAssignmentPage';

/**
 * Config assignment page: edits the single `route.v1alpha1.Router` stored under
 * the "global" key via the ResourceService, validating against the
 * CollectorService before every save. Editor / Split / Preview switch between
 * the route editor and the live assignment diff from `PreviewRouter`.
 *
 * Note: this story loads the router and the collector config list from the
 * backend. Without one running it renders empty and surfaces gRPC error
 * notifications.
 */
const meta = {
  title: 'ConfigAssignment/ConfigAssignmentPage',
  component: ConfigAssignmentPage,
} satisfies Meta<typeof ConfigAssignmentPage>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
