import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Center, Stack, Text, ThemeIcon } from '@mantine/core';
import { ExclamationTriangleIcon } from '@radix-ui/react-icons';

interface PipelineErrorBoundaryProps {
  children: ReactNode;
  // When this value changes (e.g. the config was edited), the boundary clears
  // its error state so a subsequent valid config can render again.
  resetKey?: unknown;
}

interface PipelineErrorBoundaryState {
  hasError: boolean;
  resetKey: unknown;
}

// Failures are contained to the graph pane.
export class PipelineErrorBoundary extends Component<
  PipelineErrorBoundaryProps,
  PipelineErrorBoundaryState
> {
  state: PipelineErrorBoundaryState = { hasError: false, resetKey: this.props.resetKey };

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  static getDerivedStateFromProps(
    props: PipelineErrorBoundaryProps,
    state: PipelineErrorBoundaryState,
  ): PipelineErrorBoundaryState | null {
    if (props.resetKey !== state.resetKey) {
      return { hasError: false, resetKey: props.resetKey };
    }
    return null;
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Pipeline visualization crashed:', error, info.componentStack);
  }

  render() {
    if (this.state.hasError) {
      return (
        <Center h="100%" p="md">
          <Stack align="center" gap="xs" maw={320}>
            <ThemeIcon variant="light" color="red" size="xl" radius="xl">
              <ExclamationTriangleIcon />
            </ThemeIcon>
            <Text fw={600}>Hit an unrecoverable error</Text>
            <Text size="sm" c="dimmed" ta="center">
              The pipeline visualization couldn't be rendered. Edit the configuration to recover.
            </Text>
          </Stack>
        </Center>
      );
    }
    return this.props.children;
  }
}
