import type { Meta, StoryObj } from '@storybook/tanstack-react';
import PipelineGraph from './Pipeline';

/**
 * `PipelineGraph` renders an OpenTelemetry Collector config (YAML string) as a
 * ReactFlow graph of receivers → processors → exporters, grouped per
 * `service.pipelines` entry. It is the "Graph" half of the config `Editor`, and
 * can be developed here in isolation by feeding it raw YAML via the `value` arg.
 *
 * Invalid or empty YAML falls back to the empty-state node.
 */
const meta = {
  title: 'Config/PipelineGraph',
  component: PipelineGraph,
  // ReactFlow needs an explicitly sized parent to lay out and fit the view.
  decorators: [
    (Story) => (
      <div style={{ height: 500, width: '100%' }}>
        <Story />
      </div>
    ),
  ],
  parameters: {
    layout: 'fullscreen',
  },
} satisfies Meta<typeof PipelineGraph>;

export default meta;

type Story = StoryObj<typeof meta>;

const SINGLE_PIPELINE = `receivers:
  otlp:
    protocols:
      grpc: {}

processors:
  batch: {}

exporters:
  otlphttp:
    endpoint: https://otel-collector.example.com

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp]
`;

const MULTIPLE_PIPELINES = `receivers:
  otlp:
    protocols:
      grpc: {}
      http: {}
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

/** A single traces pipeline: one receiver, one processor, one exporter. */
export const SinglePipeline: Story = {
  args: {
    value: SINGLE_PIPELINE,
  },
};

/** Traces, metrics, and logs pipelines sharing receivers and exporters. */
export const MultiplePipelines: Story = {
  args: {
    value: MULTIPLE_PIPELINES,
  },
};

/** Empty input renders the empty-state node. */
export const EmptyState: Story = {
  args: {
    value: '',
  },
};

/** Malformed YAML also falls back to the empty-state node instead of crashing. */
export const InvalidYaml: Story = {
  args: {
    value: 'receivers: [unclosed',
  },
};
