import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { ResourceListPage } from './ResourceListPage';
import { getEntityType } from './entityTypes';

/**
 * The resource list page: a generic table over one first-class resource type,
 * driven by `ResourceService.ListEntity`. Rows link to the resource editor and
 * expose delete. The "New" button routes to create mode.
 *
 * Note: the list posts to the ResourceService backend on mount. Without a
 * backend running the table renders empty and a gRPC error notification is
 * surfaced.
 */
const meta = {
  title: 'Resources/ResourceListPage',
  component: ResourceListPage,
  decorators: [
    (Story) => (
      <div style={{ padding: 16 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof ResourceListPage>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * Listing of Receiver resources.
 */
export const Receivers: Story = {
  args: {
    entityType: getEntityType('receiver')!,
  },
};

/**
 * Listing of CollectorConfig resources.
 */
export const CollectorConfigs: Story = {
  args: {
    entityType: getEntityType('collectorconfig')!,
  },
};
