import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { ConfigFilterPage } from './ConfigFilterPage';

/**
 * Config assignments page: lists ConfigFilter resources from the ResourceService
 * and creates/edits them through a modal form.
 *
 * Note: the page calls ListEntity on mount. Without a backend running the table
 * renders empty and a gRPC error notification is surfaced.
 */
const meta = {
  title: 'ConfigFilters/ConfigFilterPage',
  component: ConfigFilterPage,
  decorators: [
    (Story) => (
      <div style={{ padding: 16 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof ConfigFilterPage>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
