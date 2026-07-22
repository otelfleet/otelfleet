import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { ResourceEditor } from './ResourceEditor';
import { getEntityType } from './entityTypes';

/**
 * The resource editor: a YAML Monaco editor for a single first-class resource
 * (Receiver, Processor, CollectorConfig, ...) backed by the `ResourceService`
 * CRUD API. Content is stored as the raw value of the entity.
 *
 * For `CollectorConfig` (which sets `visualize`) the editor also renders the
 * live `PipelineGraph` and exposes the Editor / Split / Graph view toggle.
 *
 * Note: loading (edit mode) and saving hit the ResourceService backend. Without
 * a backend running these surface a gRPC error notification — the editor and
 * graph themselves work offline.
 */
const meta = {
  title: 'Resources/ResourceEditor',
  component: ResourceEditor,
  decorators: [
    (Story) => (
      <div style={{ height: '90vh', padding: 16, boxSizing: 'border-box' }}>
        <Story />
      </div>
    ),
  ],
  parameters: {
    layout: 'fullscreen',
  },
} satisfies Meta<typeof ResourceEditor>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * Create-mode editor for a component-style entity (Receiver). Raw YAML only,
 * no graph visualization.
 */
export const NewReceiver: Story = {
  args: {
    entityType: getEntityType('receiver')!,
  },
};

/**
 * Create-mode editor for a CollectorConfig. Because collector configs opt into
 * visualization, the toolbar shows the Editor / Split / Graph toggle and the
 * pipeline graph renders alongside the editor.
 */
export const NewCollectorConfig: Story = {
  args: {
    entityType: getEntityType('collectorconfig')!,
  },
};
