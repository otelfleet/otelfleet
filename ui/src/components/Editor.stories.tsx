import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { Editor } from './Editor';

/**
 * The config editor: a YAML Monaco editor paired with a live pipeline graph
 * (see `PipelineGraph`). Use the `View` control in the toolbar to switch
 * between Editor / Split / Graph.
 *
 * Note: saving a config posts to the ConfigService backend
 * (http://localhost:16587). Without a backend running, submitting surfaces a
 * gRPC error notification — the editor and graph themselves work offline.
 */
const meta = {
  title: 'Config/Editor',
  component: Editor,
  // The editor sizes itself to its container height, so give stories a
  // bounded, full-height canvas to render into.
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
  argTypes: {
    readOnly: { control: 'boolean' },
    height: { control: 'text' },
    configId: { control: 'text' },
  },
} satisfies Meta<typeof Editor>;

export default meta;

type Story = StoryObj<typeof meta>;

const SAMPLE_CONFIG = `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  hostmetrics:
    collection_interval: 30s

processors:
  batch: {}
  memory_limiter:
    check_interval: 1s
    limit_mib: 512

exporters:
  otlphttp:
    endpoint: https://otel-collector.example.com
  debug:
    verbosity: detailed

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlphttp]
    metrics:
      receivers: [otlp, hostmetrics]
      processors: [batch]
      exporters: [otlphttp, debug]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`;

/**
 * Create-mode editor: empty config name, blank document ready to author a new
 * config. This is the `/editor` route with no `configId`.
 */
export const New: Story = {
  args: {
    defaultConfig: '',
  },
};

/**
 * Edit-mode editor prefilled with an existing config. The config name is fixed
 * (derived from `configId`) and the toolbar reads "Update config".
 */
export const Editing: Story = {
  args: {
    configId: 'edge-collector',
    defaultConfig: SAMPLE_CONFIG,
  },
};

/**
 * Read-only embedding, as used on the agent detail page to show an agent's
 * effective configuration. No form, no save button — just the view toggle at a
 * fixed height.
 */
export const ReadOnly: Story = {
  args: {
    defaultConfig: SAMPLE_CONFIG,
    readOnly: true,
    height: 500,
  },
};
