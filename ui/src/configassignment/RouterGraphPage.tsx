import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { Box, Button, Group, Loader, Paper, Title } from '@mantine/core';
import { Pencil1Icon } from '@radix-ui/react-icons';
import { Panel } from 'reactflow';
import { Code, ConnectError } from '@connectrpc/connect';

import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { LabelType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import { CollectorService } from '../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';
import { ResourceService } from '../gen/api/pkg/api/resources/v1alpha1/resources_pb';
import { RouteDetailsPanel } from './RouteDetailsPanel';
import { RouteMatchQuery, labelsByType, type LabelQueryRow } from './RouteMatchQuery';
import { RouterGraph } from './RouterGraph';
import { DEFAULT_NODE_ID, nodeIdFromIndexPath, type RouteNodeData } from './routeGraph';
import {
  ROUTER_KEY,
  ROUTER_TYPE_URL,
  emptyRouterValues,
  toRouter,
  toRouterValues,
  unpackRouter,
  type RouterValues,
} from './router';

export function RouterGraphPage() {
  const resources = useClient(ResourceService);
  const collectors = useClient(CollectorService);
  const navigate = useNavigate();

  const [values, setValues] = useState<RouterValues | null>(null);
  const [assignment, setAssignment] = useState<{ [key: string]: string }>({});

  const [selected, setSelected] = useState<{ id: string; data: RouteNodeData } | null>(null);

  const [queryRows, setQueryRows] = useState<LabelQueryRow[]>([]);
  const [matchedNodeId, setMatchedNodeId] = useState<string | undefined>(undefined);
  const [matching, setMatching] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const response = await resources.getEntity({ typeUrl: ROUTER_TYPE_URL, key: ROUTER_KEY });
        if (!cancelled) setValues(toRouterValues(unpackRouter(response.entity?.obj)));
      } catch (error) {
        if (cancelled) return;
        if (ConnectError.from(error).code === Code.NotFound) {
          navigate({ to: '/configfilter/editor', replace: true });
          return;
        }
        setValues(emptyRouterValues());
        notifyGRPCError('Failed to load config assignment', error);
      }
    })();
    return () => { cancelled = true; };
  }, [resources, navigate]);

  useEffect(() => {
    if (!values) return;
    let cancelled = false;
    (async () => {
      try {
        const response = await collectors.previewRouter({ router: toRouter(values) });
        if (!cancelled) setAssignment(response.new?.collectorsToConfigRef ?? {});
      } catch (error) {
        notifyGRPCError('Failed to resolve collector assignments', error);
      }
    })();
    return () => { cancelled = true; };
  }, [collectors, values]);

  const assignedConfigRefs = useMemo(() => Object.values(assignment), [assignment]);

  const selectedCollectors = useMemo(() => {
    if (!selected || selected.data.configRef === '') return [];
    return Object.entries(assignment)
      .filter(([, configRef]) => configRef === selected.data.configRef)
      .map(([collector]) => collector)
      .sort();
  }, [assignment, selected]);

  const handleMatch = useCallback(async () => {
    if (!values) return;
    if (queryRows.length === 0) {
      setMatchedNodeId(undefined);
      return;
    }
    setMatching(true);
    try {
      const response = await collectors.matchRouter({
        router: toRouter(values),
        identifyingLabels: labelsByType(queryRows, LabelType.LabelTypeIdentifying),
        nonIdentifyingLabels: labelsByType(queryRows, LabelType.LabelTypeNonIdentifying),
        otelfleetLabels: labelsByType(queryRows, LabelType.LabelTypeOtelfleet),
      });
      setMatchedNodeId(
        response.matched
          ? nodeIdFromIndexPath(response.routeIndexPath.map(Number))
          : DEFAULT_NODE_ID,
      );
    } catch (error) {
      notifyGRPCError('Failed to match labels against the router', error);
    } finally {
      setMatching(false);
    }
  }, [values, collectors, queryRows]);

  useEffect(() => { handleMatch(); }, [handleMatch]);

  return (
    <Box style={{ display: 'flex', flexDirection: 'column', flex: 1, height: '100%', minHeight: 0, gap: 16 }}>
      <Group justify="space-between">
        <Title order={3}>Config Assignment</Title>
        <Link to="/configfilter/editor">
          <Button leftSection={<Pencil1Icon />}>Edit</Button>
        </Link>
      </Group>

      <Box style={{ flex: 1, minHeight: 0, display: 'flex', gap: 16 }}>
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
          {values ? (
            <RouterGraph
              value={values}
              assignedConfigRefs={assignedConfigRefs}
              selectedNodeId={selected?.id}
              matchedNodeId={matchedNodeId}
              onSelect={(id, data) => setSelected({ id, data })}
            >
              <Panel position="top-left">
                <RouteMatchQuery
                  rows={queryRows}
                  loading={matching}
                  onChange={setQueryRows}
                />
              </Panel>
            </RouterGraph>
          ) : (
            <Loader m="md" />
          )}
        </Paper>

        {selected && <RouteDetailsPanel route={selected.data} collectors={selectedCollectors} />}
      </Box>
    </Box>
  );
}
