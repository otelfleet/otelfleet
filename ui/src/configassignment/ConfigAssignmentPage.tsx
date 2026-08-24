import { useCallback, useEffect, useRef, useState } from 'react';
import { Box, Button, Group, Loader, Paper, SegmentedControl, Title } from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { Code, ConnectError } from '@connectrpc/connect';
import { CheckCircledIcon } from '@radix-ui/react-icons';

import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { CollectorService } from '../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';
import { ResourceService } from '../gen/api/pkg/api/resources/v1alpha1/resources_pb';
import { getEntityType } from '../resources/entityTypes';
import { RouterForm } from './RouterForm';
import { RouterPreview } from './RouterPreview';
import {
  ROUTER_KEY,
  ROUTER_TYPE_URL,
  diffAssignments,
  emptyRouterValues,
  packRouter,
  toRouter,
  toRouterValues,
  unpackRouter,
  type AssignmentChange,
  type RouterValues,
} from './router';

type ViewMode = 'editor' | 'split' | 'preview';

const PREVIEW_DEBOUNCE_MS = 400;

export function ConfigAssignmentPage() {
  const resources = useClient(ResourceService);
  const collectors = useClient(CollectorService);
  const collectorConfigType = getEntityType('collectorconfig')!;

  const [values, setValues] = useState<RouterValues | null>(null);
  const [collectorConfigs, setCollectorConfigs] = useState<string[]>([]);
  const [viewMode, setViewMode] = useState<ViewMode>('split');
  const [saving, setSaving] = useState(false);

  const [changes, setChanges] = useState<AssignmentChange[]>([]);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewError, setPreviewError] = useState<string | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const response = await resources.listEntity({ typeUrl: collectorConfigType.typeUrl });
        if (!cancelled) setCollectorConfigs(response.entities.map((e) => e.key));
      } catch (error) {
        notifyGRPCError('Failed to list collector configs', error);
      }
    })();
    return () => { cancelled = true; };
  }, [resources, collectorConfigType]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const response = await resources.getEntity({ typeUrl: ROUTER_TYPE_URL, key: ROUTER_KEY });
        if (!cancelled) setValues(toRouterValues(unpackRouter(response.entity?.obj)));
      } catch (error) {
        if (!cancelled) setValues(emptyRouterValues());
        if (ConnectError.from(error).code !== Code.NotFound) {
          notifyGRPCError('Failed to load config assignment', error);
        }
      }
    })();
    return () => { cancelled = true; };
  }, [resources]);

  const showEditor = viewMode === 'editor' || viewMode === 'split';
  const showPreview = viewMode === 'preview' || viewMode === 'split';

  const refreshPreview = useRefreshPreview({
    values,
    enabled: showPreview,
    collectors,
    setChanges,
    setPreviewLoading,
    setPreviewError,
  });

  const handleSubmit = useCallback(async () => {
    if (!values) return;
    setSaving(true);
    try {
      await collectors.validateRouter({ router: toRouter(values) });
      await resources.putEntity({
        entity: { typeUrl: ROUTER_TYPE_URL, key: ROUTER_KEY, obj: packRouter(values) },
      });
      notifications.show({
        title: 'Config assignment saved',
        message: 'Collectors will pick up their configs on the next sync',
        icon: <CheckCircledIcon />,
      });
      refreshPreview();
    } catch (error) {
      notifyGRPCError('Failed to save config assignment', error);
    } finally {
      setSaving(false);
    }
  }, [values, collectors, resources, refreshPreview]);

  return (
    <Box style={{ display: 'flex', flexDirection: 'column', flex: 1, height: '100%', minHeight: 0, gap: 16 }}>
      <Group align="flex-end" gap="md">
        <Title order={3}>Config Assignment</Title>
        <SegmentedControl
          value={viewMode}
          onChange={(value) => setViewMode(value as ViewMode)}
          data={[
            { label: 'Editor', value: 'editor' },
            { label: 'Split', value: 'split' },
            { label: 'Preview', value: 'preview' },
          ]}
        />
        <Button loading={saving} onClick={handleSubmit}>Save</Button>
      </Group>

      {values ? (
        <Box style={{ flex: 1, minHeight: 0, display: 'flex', gap: 16 }}>
          {showEditor && (
            <Paper shadow="sm" radius="md" p="md" style={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
              <RouterForm value={values} collectorConfigs={collectorConfigs} onChange={setValues} />
            </Paper>
          )}

          {showPreview && (
            <Paper shadow="sm" radius="md" p="md" style={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
              <RouterPreview changes={changes} loading={previewLoading} error={previewError} />
            </Paper>
          )}
        </Box>
      ) : (
        <Loader />
      )}
    </Box>
  );
}

interface RefreshPreviewOptions {
  values: RouterValues | null;
  enabled: boolean;
  collectors: ReturnType<typeof useClient<typeof CollectorService>>;
  setChanges: (changes: AssignmentChange[]) => void;
  setPreviewLoading: (loading: boolean) => void;
  setPreviewError: (error: string | undefined) => void;
}

// Previews the edited router against the live fleet, debounced while typing.
function useRefreshPreview(opts: RefreshPreviewOptions) {
  const { values, enabled, collectors, setChanges, setPreviewLoading, setPreviewError } = opts;
  const generation = useRef(0);

  const refresh = useCallback(async () => {
    if (!values || !enabled) return;
    const current = ++generation.current;
    setPreviewLoading(true);
    try {
      const response = await collectors.previewRouter({ router: toRouter(values) });
      if (current !== generation.current) return;
      setPreviewError(undefined);
      setChanges(
        diffAssignments(
          response.old?.collectorsToConfigRef ?? {},
          response.new?.collectorsToConfigRef ?? {},
        ),
      );
    } catch (error) {
      if (current !== generation.current) return;
      setChanges([]);
      setPreviewError(ConnectError.from(error).message);
    } finally {
      if (current === generation.current) setPreviewLoading(false);
    }
  }, [values, enabled, collectors, setChanges, setPreviewLoading, setPreviewError]);

  useEffect(() => {
    if (!enabled) return;
    const timer = setTimeout(refresh, PREVIEW_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [enabled, refresh]);

  return refresh;
}
