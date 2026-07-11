import type { Preview } from '@storybook/tanstack-react'
import { MantineProvider } from '@mantine/core'

// Match the app's global style imports (see src/main.tsx)
import '@mantine/core/styles.css'
import '@mantine/notifications/styles.css'

const preview: Preview = {
  decorators: [
    (Story) => (
      <MantineProvider>
        <Story />
      </MantineProvider>
    ),
  ],
  parameters: {
    controls: {
      matchers: {
       color: /(background|color)$/i,
       date: /Date$/i,
      },
    },

    a11y: {
      // 'todo' - show a11y violations in the test UI only
      // 'error' - fail CI on a11y violations
      // 'off' - skip a11y checks entirely
      test: 'todo'
    }
  },
};

export default preview;