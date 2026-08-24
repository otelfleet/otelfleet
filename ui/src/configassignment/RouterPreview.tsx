import { Alert, Badge, Center, Loader, Stack, Table, Text } from '@mantine/core';
import { CrossCircledIcon } from '@radix-ui/react-icons';

import type { AssignmentChange, AssignmentChangeKind } from './router';

const CHANGE_BADGES: Record<AssignmentChangeKind, { color: string; label: string } | null> = {
  unchanged: null,
  changed: { color: 'yellow', label: 'changed' },
  assigned: { color: 'green', label: 'new' },
  unassigned: { color: 'red', label: 'unassigned' },
};

interface RouterPreviewProps {
  changes: AssignmentChange[];
  loading?: boolean;
  error?: string;
}

export function RouterPreview({ changes, loading = false, error }: RouterPreviewProps) {
  if (error) {
    return (
      <Alert color="red" icon={<CrossCircledIcon />} title="Config assignment is invalid">
        {error}
      </Alert>
    );
  }

  if (loading && changes.length === 0) {
    return (
      <Center h="100%">
        <Loader />
      </Center>
    );
  }

  if (changes.length === 0) {
    return <Text size="sm" c="dimmed">No collectors are currently connected.</Text>;
  }

  const changed = changes.filter((change) => change.kind !== 'unchanged');

  return (
    <Stack gap="sm" opacity={loading ? 0.6 : 1}>
      <Text size="sm">
        {changed.length} of {changes.length} collectors change config.
      </Text>

      <Table>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Collector</Table.Th>
            <Table.Th>Current</Table.Th>
            <Table.Th>New</Table.Th>
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {changes.map((change) => {
            const badge = CHANGE_BADGES[change.kind];
            return (
              <Table.Tr key={change.collector}>
                <Table.Td>{change.collector}</Table.Td>
                <Table.Td>
                  <ConfigRef value={change.oldConfigRef} />
                </Table.Td>
                <Table.Td>
                  <ConfigRef value={change.newConfigRef} />
                </Table.Td>
                <Table.Td>
                  {badge && <Badge color={badge.color} variant="light">{badge.label}</Badge>}
                </Table.Td>
              </Table.Tr>
            );
          })}
        </Table.Tbody>
      </Table>
    </Stack>
  );
}

function ConfigRef({ value }: { value: string }) {
  if (value === '') return <Text c="dimmed">none</Text>;
  return <Text size="sm">{value}</Text>;
}
