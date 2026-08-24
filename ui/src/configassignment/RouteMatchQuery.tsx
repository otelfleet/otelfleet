import { useState } from 'react';
import { ActionIcon, Autocomplete, Badge, Button, Group, Loader, Paper, Popover, Select, Stack, Text } from '@mantine/core';
import { Cross2Icon, PlusIcon } from '@radix-ui/react-icons';

import { LabelType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import { LABEL_TYPE_NAMES } from './routeGraph';
import { LABEL_KEY_SUGGESTIONS, LABEL_TYPE_OPTIONS, LABEL_VALUE_SUGGESTIONS } from './router';

export interface LabelQueryRow {
  type: LabelType;
  key: string;
  value: string;
}

export function labelsByType(rows: LabelQueryRow[], type: LabelType): { [key: string]: string } {
  const labels: { [key: string]: string } = {};
  for (const row of rows) {
    if (row.type === type && row.key !== '') labels[row.key] = row.value;
  }
  return labels;
}

interface RouteMatchQueryProps {
  rows: LabelQueryRow[];
  loading?: boolean;
  onChange: (rows: LabelQueryRow[]) => void;
}

export function RouteMatchQuery({ rows, loading = false, onChange }: RouteMatchQueryProps) {
  const [opened, setOpened] = useState(false);

  const addFilter = (row: LabelQueryRow) => {
    onChange([...rows.filter((existing) => existing.type !== row.type || existing.key !== row.key), row]);
    setOpened(false);
  };

  return (
    <Paper withBorder shadow="sm" p={6} radius="md">
      <Group gap={6} wrap="wrap" align="center">
        <Popover opened={opened} onChange={setOpened} position="bottom-start" withArrow shadow="md">
          <Popover.Target>
            <Button size="xs" variant="default" leftSection={<PlusIcon />} onClick={() => setOpened(true)}>
              Add filter
            </Button>
          </Popover.Target>
          <Popover.Dropdown p="sm">
            <FilterEditor onAdd={addFilter} onCancel={() => setOpened(false)} />
          </Popover.Dropdown>
        </Popover>

        {rows.map((row, index) => (
          <FilterChip
            key={`${row.type}-${row.key}`}
            row={row}
            onRemove={() => onChange(rows.filter((_, i) => i !== index))}
          />
        ))}

        {loading && <Loader size="xs" />}
      </Group>
    </Paper>
  );
}

function FilterChip({ row, onRemove }: { row: LabelQueryRow; onRemove: () => void }) {
  return (
    <Badge
      variant="light"
      size="lg"
      radius="sm"
      style={{ textTransform: 'none' }}
      rightSection={
        <ActionIcon variant="transparent" color="gray" size="xs" title="Remove filter" onClick={onRemove}>
          <Cross2Icon />
        </ActionIcon>
      }
    >
      <Text span size="xs" c="dimmed">{LABEL_TYPE_NAMES[row.type]}</Text>
      <Text span size="xs" ff="monospace"> {row.key}={row.value}</Text>
    </Badge>
  );
}

interface FilterEditorProps {
  onAdd: (row: LabelQueryRow) => void;
  onCancel: () => void;
}

function FilterEditor({ onAdd, onCancel }: FilterEditorProps) {
  const [row, setRow] = useState<LabelQueryRow>({
    type: LabelType.LabelTypeIdentifying,
    key: '',
    value: '',
  });

  const submit = () => row.key !== '' && onAdd(row);

  return (
    <Stack gap="xs" w={300}>
      <Select
        size="xs"
        label="Scope"
        comboboxProps={{ withinPortal: false }}
        data={LABEL_TYPE_OPTIONS}
        value={String(row.type)}
        onChange={(next) => setRow({ ...row, type: Number(next ?? LabelType.LabelTypeIdentifying) })}
      />
      <Group gap={6} wrap="nowrap" align="flex-end">
        <Autocomplete
          size="xs"
          label="Key"
          comboboxProps={{ withinPortal: false }}
          placeholder="service.name"
          data={LABEL_KEY_SUGGESTIONS[row.type].map((s) => s.key)}
          style={{ flex: 1 }}
          value={row.key}
          onChange={(next) => setRow({ ...row, key: next })}
          onKeyDown={(event) => event.key === 'Enter' && submit()}
        />
        <Autocomplete
          size="xs"
          label="Value"
          comboboxProps={{ withinPortal: false }}
          placeholder="otelcol-contrib"
          data={LABEL_VALUE_SUGGESTIONS[row.key] ?? []}
          style={{ flex: 1 }}
          value={row.value}
          onChange={(next) => setRow({ ...row, value: next })}
          onKeyDown={(event) => event.key === 'Enter' && submit()}
        />
      </Group>
      <Group justify="flex-end" gap="xs">
        <Button size="xs" variant="subtle" onClick={onCancel}>Cancel</Button>
        <Button size="xs" disabled={row.key === ''} onClick={submit}>Add</Button>
      </Group>
    </Stack>
  );
}
