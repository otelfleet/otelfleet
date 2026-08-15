import { useEffect, useRef, useState } from 'react';
import type { Meta, StoryObj } from '@storybook/tanstack-react';
import * as monaco from 'monaco-editor';
import { Box } from '@mantine/core';
import { DiagnosticsPanel } from './DiagnosticsPanel';

interface HarnessProps {
  markers: monaco.editor.IMarkerData[];
}

// The panel reads live markers off a monaco model, so the story spins up a
// throwaway editor, seeds it with markers, and renders the panel against it.
function Harness({ markers }: HarnessProps) {
  const hostRef = useRef<HTMLDivElement>(null);
  const [session, setSession] = useState<{
    monaco: typeof monaco;
    editor: monaco.editor.IStandaloneCodeEditor;
  } | null>(null);

  useEffect(() => {
    if (!hostRef.current) return;
    const editor = monaco.editor.create(hostRef.current, {
      value: 'receivers:\n  otlp:\n    protocols:\n      grpc: {}\nservice:\n  pipelines:\n    traces:\n      receivers: [nonexistent_receiver]\n      exporters: [debug]\n',
      language: 'yaml',
      automaticLayout: true,
      minimap: { enabled: false },
    });
    const model = editor.getModel();
    if (model) monaco.editor.setModelMarkers(model, 'otelcol-lsp', markers);
    setSession({ monaco, editor });
    return () => editor.dispose();
  }, [markers]);

  return (
    <Box style={{ display: 'flex', flexDirection: 'column', gap: 16, height: '80vh' }}>
      <Box ref={hostRef} style={{ flex: 1, minHeight: 0 }} />
      {session && <DiagnosticsPanel monaco={session.monaco} editor={session.editor} />}
    </Box>
  );
}

const meta = {
  title: 'LSP/DiagnosticsPanel',
  component: Harness,
  parameters: { layout: 'fullscreen' },
  decorators: [(Story) => <Box p="md"><Story /></Box>],
} satisfies Meta<typeof Harness>;

export default meta;

type Story = StoryObj<typeof meta>;

export const WithProblems: Story = {
  args: {
    markers: [
      { severity: 8, message: 'receiver or connector "nonexistent_receiver" is not defined', startLineNumber: 8, startColumn: 18, endLineNumber: 8, endColumn: 38, source: 'otelcol-lsp' },
      { severity: 8, message: 'exporter or connector "debug" is not defined', startLineNumber: 9, startColumn: 18, endLineNumber: 9, endColumn: 23, source: 'otelcol-lsp' },
      { severity: 4, message: 'receiver "otlp" is defined but not used in any pipeline', startLineNumber: 2, startColumn: 3, endLineNumber: 2, endColumn: 7, source: 'otelcol-lsp' },
      { severity: 2, message: 'consider setting a memory_limiter processor', startLineNumber: 5, startColumn: 1, endLineNumber: 5, endColumn: 8, source: 'otelcol-lsp' },
    ],
  },
};

export const NoProblems: Story = {
  args: { markers: [] },
};
