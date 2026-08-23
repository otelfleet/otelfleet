import { memo } from 'react';
import { Handle, Position } from 'reactflow';
import { Badge, Box, Group, Paper, Stack, Text } from '@mantine/core';
import { CheckCircledIcon, DesktopIcon, FileTextIcon } from '@radix-ui/react-icons';

import type { RouteNodeData } from './routeGraph';

const handleStyle = { backgroundColor: 'transparent', borderColor: 'transparent' };

function accentColor(data: RouteNodeData): string {
  if (data.matched) return 'green';
  if (data.isDef) return 'grape';
  if (data.isRoot) return 'blue';
  return 'gray';
}

function RouteNode({ data }: { data: RouteNodeData }) {
  const accent = accentColor(data);
  const ring = data.matched
    ? 'var(--mantine-color-green-filled)'
    : data.selected
      ? 'var(--mantine-color-blue-filled)'
      : undefined;

  return (
    <Paper
      withBorder
      shadow={data.selected || data.matched ? 'md' : 'xs'}
      radius="md"
      w={260}
      style={{
        cursor: 'pointer',
        overflow: 'hidden',
        borderColor: ring ?? undefined,
        boxShadow: ring ? `0 0 0 3px color-mix(in srgb, ${ring} 25%, transparent)` : undefined,
        transition: 'box-shadow 120ms ease, transform 120ms ease',
      }}
    >
      <Handle type="target" position={Position.Left} style={handleStyle} />

      <Box
        px="xs"
        py={6}
        style={{
          borderBottom: '1px solid var(--mantine-color-default-border)',
          background: `color-mix(in srgb, var(--mantine-color-${accent}-filled) 12%, transparent)`,
        }}
      >
        <Group justify="space-between" gap={6} wrap="nowrap">
          <Group gap={6} wrap="nowrap" style={{ minWidth: 0 }}>
            <Box
              w={6}
              h={6}
              style={{ borderRadius: '50%', background: `var(--mantine-color-${accent}-filled)`, flexShrink: 0 }}
            />
            <Text size="sm" fw={600} truncate>{data.name}</Text>
          </Group>
          {data.matched ? (
            <Badge size="xs" color="green" variant="filled" leftSection={<CheckCircledIcon />}>match</Badge>
          ) : data.isRoot ? (
            <Badge size="xs" variant="light" color="blue">root</Badge>
          ) : data.isDef ? (
            <Badge size="xs" variant="light" color="grape">def</Badge>
          ) : null}
        </Group>
      </Box>

      <Stack gap={6} px="xs" py={8}>
        {data.use !== '' ? (
          <Text size="xs" c="dimmed">
            uses <Text span fw={600} c="grape">{data.use}</Text>
          </Text>
        ) : data.matchers.length === 0 ? (
          <Text size="xs" c="dimmed" fs="italic">matches every collector</Text>
        ) : (
          <Stack gap={3}>
            {data.matchers.map((matcher) => (
              <MatcherRow key={matcher} matcher={matcher} />
            ))}
          </Stack>
        )}

        <Group justify="space-between" gap="xs" wrap="nowrap">
          <Group gap={4} wrap="nowrap" style={{ minWidth: 0 }}>
            <Box c="dimmed" style={{ display: 'flex', flexShrink: 0 }}><FileTextIcon /></Box>
            {data.configRef ? (
              <Text size="xs" ff="monospace" truncate>{data.configRef}</Text>
            ) : (
              <Text size="xs" c="dimmed" fs="italic">no config</Text>
            )}
          </Group>
          <Badge
            size="xs"
            variant="light"
            color={data.collectors > 0 ? 'blue' : 'gray'}
            leftSection={<DesktopIcon />}
            title={`${data.collectors} collectors`}
          >
            {data.collectors}
          </Badge>
        </Group>
      </Stack>

      <Handle type="source" position={Position.Right} style={handleStyle} />
    </Paper>
  );
}

function MatcherRow({ matcher }: { matcher: string }) {
  const [scope, expression] = matcher.split(' | ');
  return (
    <Group gap={6} wrap="nowrap" align="center">
      <Badge size="xs" variant="default" radius="sm" style={{ flexShrink: 0, textTransform: 'none' }}>
        {scope}
      </Badge>
      <Text size="xs" ff="monospace" fz={10} truncate>{expression}</Text>
    </Group>
  );
}

export default memo(RouteNode);
