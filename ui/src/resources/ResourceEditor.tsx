import { useState, useEffect } from 'react';
import type * as monacoNs from 'monaco-editor';
import MonacoEditor, { type OnChange, type OnMount } from '@monaco-editor/react';
import { Box, Button, Group, TextInput, Paper, SegmentedControl } from '@mantine/core';
import { useForm } from '@mantine/form';
import { useNavigate } from '@tanstack/react-router';
import { notifications } from '@mantine/notifications';
import { CheckCircledIcon } from '@radix-ui/react-icons';

import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { useMonacoTheme } from '../hooks/useMonacoTheme';
import { ResourceService } from '../gen/api/pkg/api/resources/v1alpha1/resources_pb';
import PipelineGraph from '../pipelines/Pipeline';
import type { EntityType } from './entityTypes';
import { useCollectorConfigLsp } from '../lsp/useCollectorConfigLsp';
import { LspStatusBadge } from '../lsp/LspStatusBadge';
import { DiagnosticsPanel } from '../lsp/DiagnosticsPanel';

type ViewMode = 'editor' | 'graph' | 'split';

interface ResourceEditorProps {
  entityType: EntityType;
  entityKey?: string;
}

export function ResourceEditor({ entityType, entityKey }: ResourceEditorProps) {
  const isEditMode = Boolean(entityKey);
  const monacoTheme = useMonacoTheme();
  const client = useClient(ResourceService);
  const navigate = useNavigate();

  const [content, setContent] = useState(entityKey ? '' : entityType.defaultContent ?? '');
  const [viewMode, setViewMode] = useState<ViewMode>(entityType.visualize ? 'split' : 'editor');

  const lspEnabled = entityType.slug === 'collectorconfig';
  const [lspSession, setLspSession] = useState<{
    monaco: typeof monacoNs;
    editor: monacoNs.editor.IStandaloneCodeEditor;
  } | null>(null);
  const documentUri = `file:///collectorconfig/${entityKey ?? 'new'}.yaml`;
  const lspStatus = useCollectorConfigLsp(lspSession, documentUri, lspEnabled);

  const form = useForm({
    mode: 'controlled',
    initialValues: { name: entityKey ?? '' },
    validate: {
      name: (value) => (/[a-zA-Z0-9]/.test(value) ? null : 'Invalid name'),
    },
  });

  useEffect(() => {
    if (!entityKey) return;
    let cancelled = false;
    (async () => {
      try {
        const response = await client.getEntity({ typeUrl: entityType.typeUrl, key: entityKey });
        if (!cancelled) setContent(entityType.unpack(response.entity?.obj));
      } catch (error) {
        notifyGRPCError(`Failed to load ${entityType.label}`, error);
      }
    })();
    return () => { cancelled = true; };
  }, [client, entityType, entityKey]);

  const handleEditorChange: OnChange = (value) => {
    if (value !== undefined) setContent(value);
  };

  const handleEditorMount: OnMount = (mountedEditor, monaco) => {
    if (lspEnabled) setLspSession({ monaco, editor: mountedEditor });
    const node = mountedEditor.getContainerDomNode();
    const observer = new ResizeObserver(() => {
      const { width, height } = node.getBoundingClientRect();
      if (width === 0 || height === 0) return;
      mountedEditor.layout();
      mountedEditor.setScrollTop(0);
      observer.disconnect();
    });
    observer.observe(node);
  };

  const handleSubmit = async (values: { name: string }) => {
    try {
      await client.putEntity({
        entity: {
          typeUrl: entityType.typeUrl,
          key: values.name,
          obj: entityType.pack(content),
        },
      });
      notifications.show({
        title: isEditMode ? `${entityType.label} updated` : `${entityType.label} created`,
        message: `${isEditMode ? 'Updated' : 'Created'} "${values.name}"`,
        icon: <CheckCircledIcon />,
      });
      navigate({ to: '/resources/$type', params: { type: entityType.slug } });
    } catch (error) {
      notifyGRPCError(isEditMode ? `Failed to update ${entityType.label}` : `Failed to create ${entityType.label}`, error);
    }
  };

  const showEditor = viewMode === 'editor' || viewMode === 'split';
  const showGraph = entityType.visualize && (viewMode === 'graph' || viewMode === 'split');

  return (
    <Box
      style={{
        display: 'flex',
        flexDirection: 'column',
        flex: 1,
        height: '100%',
        minHeight: 0,
        gap: 16,
      }}
    >
      <form onSubmit={form.onSubmit(handleSubmit)}>
        <Group align="flex-end" gap="md">
          <TextInput
            withAsterisk
            label={`${entityType.label} name`}
            placeholder="name"
            disabled={isEditMode}
            {...form.getInputProps('name')}
          />
          {entityType.visualize && (
            <SegmentedControl
              value={viewMode}
              onChange={(value) => setViewMode(value as ViewMode)}
              data={[
                { label: 'Editor', value: 'editor' },
                { label: 'Split', value: 'split' },
                { label: 'Graph', value: 'graph' },
              ]}
            />
          )}
          <Button type="submit" leftSection={<entityType.icon />}>{isEditMode ? `Update ${entityType.label}` : `Save ${entityType.label}`}</Button>
          {lspEnabled && <LspStatusBadge status={lspStatus} />}
        </Group>
      </form>

      <Box style={{ flex: 1, minHeight: 0, display: 'flex', gap: 16 }}>
        {showEditor && (
          <Paper shadow="sm" radius="md" style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
            <MonacoEditor
              value={content}
              width="100%"
              height="100%"
              defaultLanguage="yaml"
              theme={monacoTheme}
              options={{
                quickSuggestions: { other: true, strings: true },
                automaticLayout: true,
                scrollBeyondLastLine: false,
                minimap: { enabled: false },
                scrollbar: { verticalScrollbarSize: 8, horizontal: 'hidden' },
                padding: { top: 5 },
                fontSize: 13,
                fontWeight: '400',
              }}
              onMount={handleEditorMount}
              onChange={handleEditorChange}
            />
          </Paper>
        )}

        {showGraph && (
          <Paper
            shadow="sm"
            radius="md"
            style={{
              flex: 1,
              minHeight: 0,
              overflow: 'hidden',
              backgroundColor: 'var(--elevation-surface-bg)',
            }}
          >
            <PipelineGraph key={viewMode} value={content} />
          </Paper>
        )}
      </Box>

      {lspEnabled && lspSession && (
        <DiagnosticsPanel monaco={lspSession.monaco} editor={lspSession.editor} />
      )}
    </Box>
  );
}
