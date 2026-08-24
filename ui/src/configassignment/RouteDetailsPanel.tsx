import { Link } from '@tanstack/react-router';
import { Anchor, Badge, Code, Divider, Group, Paper, ScrollArea, Stack, Text } from '@mantine/core';

import type { RouteNodeData } from './routeGraph';

interface RouteDetailsPanelProps {
  route: RouteNodeData;
  collectors: string[];
}

export function RouteDetailsPanel({ route, collectors }: RouteDetailsPanelProps) {
  return (
    <Paper shadow="sm" radius="md" p="md" w={340} style={{ minHeight: 0, display: 'flex' }}>
      <Stack gap="sm" style={{ flex: 1, minHeight: 0 }}>
        <Group justify="space-between" gap="xs">
          <Text fw={600}>{route.name}</Text>
          {route.isRoot && <Badge size="sm" variant="light">root</Badge>}
          {route.isDef && <Badge size="sm" variant="light" color="grape">def</Badge>}
        </Group>

        <Stack gap={4}>
          <Text size="xs" c="dimmed">Collector config</Text>
          <Text size="sm">{route.configRef || 'none'}</Text>
        </Stack>

        <Stack gap={4}>
          <Text size="xs" c="dimmed">Matchers</Text>
          {route.use !== '' ? (
            <Text size="sm">uses {route.use}</Text>
          ) : route.matchers.length === 0 ? (
            <Text size="sm">matches every collector</Text>
          ) : (
            route.matchers.map((matcher) => <Code key={matcher} fz={11}>{matcher}</Code>)
          )}
        </Stack>

        <Divider />

        <Text size="xs" c="dimmed">Assigned collectors ({collectors.length})</Text>
        <ScrollArea style={{ flex: 1, minHeight: 0 }}>
          <Stack gap={4}>
            {collectors.length === 0 ? (
              <Text size="sm" c="dimmed">No collectors are assigned this config.</Text>
            ) : (
              collectors.map((collector) => (
                <Link
                  key={collector}
                  to="/deployments/$agentId"
                  params={{ agentId: collector }}
                  style={{ textDecoration: 'none' }}
                >
                  <Anchor component="span" size="xs" ff="monospace">{collector}</Anchor>
                </Link>
              ))
            )}
          </Stack>
        </ScrollArea>
      </Stack>
    </Paper>
  );
}
