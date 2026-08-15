import { useEffect, useState } from 'react';
import type * as monacoNs from 'monaco-editor';
import { Box, Group, Paper, ScrollArea, Text, UnstyledButton } from '@mantine/core';

interface DiagnosticsPanelProps {
  monaco: typeof monacoNs;
  editor: monacoNs.editor.IStandaloneCodeEditor;
}

const SEVERITY: Record<number, { label: string; color: string }> = {
  8: { label: 'error', color: 'var(--mantine-color-red-6)' },
  4: { label: 'warning', color: 'var(--mantine-color-yellow-6)' },
  2: { label: 'info', color: 'var(--mantine-color-blue-5)' },
  1: { label: 'hint', color: 'var(--mantine-color-gray-5)' },
};

function markerCode(code: monacoNs.editor.IMarker['code']): string | undefined {
  if (code === undefined) return undefined;
  return typeof code === 'string' ? code : code.value;
}

export function DiagnosticsPanel({ monaco, editor }: DiagnosticsPanelProps) {
  const [markers, setMarkers] = useState<Array<monacoNs.editor.IMarker>>([]);

  useEffect(() => {
    const model = editor.getModel();
    if (!model) return;
    const refresh = () => setMarkers(monaco.editor.getModelMarkers({ resource: model.uri }));
    refresh();
    const listener = monaco.editor.onDidChangeMarkers((uris) => {
      if (uris.some((uri) => uri.toString() === model.uri.toString())) refresh();
    });
    return () => listener.dispose();
  }, [monaco, editor]);

  const goTo = (marker: monacoNs.editor.IMarker) => {
    editor.setPosition({ lineNumber: marker.startLineNumber, column: marker.startColumn });
    editor.revealLineInCenter(marker.startLineNumber);
    editor.focus();
  };

  return (
    <Paper shadow="sm" radius="md" withBorder style={{ overflow: 'hidden' }}>
      <Group
        justify="space-between"
        px="sm"
        py={6}
        style={{ borderBottom: '1px solid var(--mantine-color-default-border)' }}
      >
        <Text size="xs" fw={600} tt="uppercase" c="dimmed">
          Problems
        </Text>
        <Text size="xs" c="dimmed">{markers.length}</Text>
      </Group>

      {markers.length === 0 ? (
        <Box px="sm" py="xs">
          <Text size="xs" c="dimmed">No problems detected.</Text>
        </Box>
      ) : (
        <ScrollArea.Autosize mah={160} type="auto">
          {markers.map((marker, index) => {
            const severity = SEVERITY[marker.severity] ?? SEVERITY[8];
            const code = markerCode(marker.code);
            return (
              <UnstyledButton
                key={index}
                onClick={() => goTo(marker)}
                px="sm"
                py={4}
                style={{
                  display: 'block',
                  width: '100%',
                  borderBottom: '1px solid var(--mantine-color-default-border)',
                }}
                __vars={{ '--hover-bg': 'var(--mantine-color-default-hover)' }}
                className="lsp-diagnostic-row"
              >
                <Group gap="xs" wrap="nowrap" align="flex-start">
                  <Text size="xs" ff="monospace" style={{ color: severity.color, minWidth: 52 }}>
                    {severity.label}
                  </Text>
                  <Text size="xs" ff="monospace" style={{ flex: 1, wordBreak: 'break-word' }}>
                    {marker.message}
                  </Text>
                  {code && <Text size="xs" ff="monospace" c="dimmed">{code}</Text>}
                  <Text size="xs" ff="monospace" c="dimmed" style={{ whiteSpace: 'nowrap' }}>
                    [{marker.startLineNumber}:{marker.startColumn}]
                  </Text>
                </Group>
              </UnstyledButton>
            );
          })}
        </ScrollArea.Autosize>
      )}
      <style>{'.lsp-diagnostic-row:hover{background:var(--hover-bg)}'}</style>
    </Paper>
  );
}
