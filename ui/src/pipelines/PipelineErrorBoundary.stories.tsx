import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { Box, Text } from '@mantine/core';
import { PipelineErrorBoundary } from './PipelineErrorBoundary';

function Exploding(): React.ReactElement {
  throw new Error('boom: pipeline layout failed');
}

interface DemoProps {
  crash: boolean;
}

// Functional wrapper so Storybook can type the args cleanly (the boundary
// itself is a class component).
function Demo({ crash }: DemoProps) {
  return (
    <Box style={{ height: 320, width: 480, border: '1px solid var(--mantine-color-default-border)' }}>
      <PipelineErrorBoundary resetKey={crash}>
        {crash ? <Exploding /> : <Text>Pipeline rendered fine.</Text>}
      </PipelineErrorBoundary>
    </Box>
  );
}

const meta = {
  title: 'Pipeline/PipelineErrorBoundary',
  component: Demo,
  argTypes: { crash: { control: 'boolean' } },
} satisfies Meta<typeof Demo>;

export default meta;

type Story = StoryObj<typeof meta>;

/** A child that throws renders the "unrecoverable error" fallback. */
export const Errored: Story = { args: { crash: true } };

/** A healthy child renders normally. */
export const Healthy: Story = { args: { crash: false } };
