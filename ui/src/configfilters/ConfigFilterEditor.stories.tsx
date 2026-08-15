import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { ConfigFilterEditor } from './ConfigFilterEditor';

/**
 * Full-page editor for one config assignment, reached from the config
 * assignments list. Create mode starts blank; edit mode loads the resource by
 * key through `ResourceService.GetEntity`.
 *
 * Note: the editor calls the ResourceService on mount. Without a backend
 * running the collector config options stay empty and a gRPC error
 * notification is surfaced.
 */
const meta = {
  title: 'ConfigFilters/ConfigFilterEditor',
  component: ConfigFilterEditor,
  decorators: [
    (Story) => (
      <div style={{ padding: 16, maxWidth: 900 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof ConfigFilterEditor>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Create: Story = {};

export const Edit: Story = {
  args: {
    entityKey: 'production-linux',
  },
};
