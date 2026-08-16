import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { fn } from 'storybook/test';
import { TokenForm } from './TokenForm';
import { emptyTokenValues } from './token';

/**
 * Form used to create a bootstrap token. Labels defined here are carried by the
 * agents that bootstrap with the token, and are what config filters match on.
 */
const meta = {
  title: 'Tokens/TokenForm',
  component: TokenForm,
  args: {
    initialValues: emptyTokenValues(),
    onSubmit: fn(),
    onCancel: fn(),
  },
  decorators: [
    (Story) => (
      <div style={{ padding: 16, maxWidth: 900 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof TokenForm>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {};

export const WithLabels: Story = {
  args: {
    initialValues: {
      ttlSeconds: 3600,
      labels: [
        { key: 'environment', value: 'production' },
        { key: 'region', value: 'us-east-1' },
      ],
    },
  },
};
