import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { LspStatusBadge } from './LspStatusBadge';

/**
 * Compact badge that surfaces the CollectorConfig editor's LSP WebSocket
 * connection status. Rendered in the editor toolbar so authors can tell whether
 * live diagnostics are available.
 */
const meta = {
  title: 'LSP/StatusBadge',
  component: LspStatusBadge,
  argTypes: {
    status: {
      control: 'select',
      options: ['connecting', 'connected', 'disconnected', 'error'],
    },
  },
} satisfies Meta<typeof LspStatusBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Connecting: Story = { args: { status: 'connecting' } };
export const Connected: Story = { args: { status: 'connected' } };
export const Disconnected: Story = { args: { status: 'disconnected' } };
export const Error: Story = { args: { status: 'error' } };
