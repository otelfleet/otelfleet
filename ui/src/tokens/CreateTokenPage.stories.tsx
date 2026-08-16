import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { CreateTokenPage } from './CreateTokenPage';

/**
 * Full-page token creation, reached from the tokens list. On submit it calls
 * `TokenService.CreateToken` and navigates to the new token's detail page.
 *
 * Note: without a backend running, submitting surfaces a gRPC error
 * notification.
 */
const meta = {
  title: 'Tokens/CreateTokenPage',
  component: CreateTokenPage,
  decorators: [
    (Story) => (
      <div style={{ padding: 16, maxWidth: 900 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CreateTokenPage>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Create: Story = {};
