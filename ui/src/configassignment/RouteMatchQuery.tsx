import { useState } from 'react';
import { ActionIcon, Alert, Autocomplete, Badge, Button, Group, Paper, ScrollArea, Select, Stack, Text } from '@mantine/core';
import { Cross2Icon, MagnifyingGlassIcon, PlusIcon, TrashIcon } from '@radix-ui/react-icons';

import { LabelType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import { LABEL_KEY_SUGGESTIONS, LABEL_TYPE_OPTIONS, LABEL_VALUE_SUGGESTIONS } from './router';

export interface LabelQueryRow {
  type: LabelType;
  key: string;
  value: string;
}

export interface RouteMatchResult {
  path: string[];
  configRef: string;
  matched: boolean;
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
  result?: RouteMatchResult;
  loading?: boolean;
  onChange: (rows: LabelQueryRow[]) => void;
  onMatch: () => void;
}

export function RouteMatchQuery({ rows, result, loading = false, onChange, onMatch }: RouteMatchQueryProps) {
  const [opened, setOpened] = useState(false);

  if (!opened) {
    return (
      <Button
        size="xs"
        variant="default"
        leftSection={<MagnifyingGlassIcon />}
        onClick={() => setOpened(true)}
      >
        Find route by labels
      </Button>
    );
  }

  const updateRow = (index: number, row: Partial<LabelQueryRow>) =>
    onChange(rows.map((existing, i) => (i === index ? { ...existing, ...row } : existing)));

  return (
    <Paper withBorder shadow="md" p="sm" w={320}>
      <Stack gap="xs">
        <Group justify="space-between" gap="xs">
          <Text size="sm" fw={600}>Find route by labels</Text>
          <ActionIcon variant="subtle" size="sm" title="Close" onClick={() => setOpened(false)}>
            <Cross2Icon />
          </ActionIcon>
        </Group>

        <ScrollArea.Autosize mah={240}>
          <Stack gap="xs">
            {rows.map((row, index) => (
              <Group key={index} align="flex-end" gap={6} wrap="nowrap">
                <Stack gap={4} style={{ flex: 1 }}>
                  <Select
                    size="xs"
                    data={LABEL_TYPE_OPTIONS}
                    value={String(row.type)}
                    onChange={(next) => updateRow(index, { type: Number(next ?? LabelType.LabelTypeIdentifying) })}
                  />
                  <Group gap={6} wrap="nowrap">
                    <Autocomplete
                      size="xs"
                      placeholder="service.name"
                      data={LABEL_KEY_SUGGESTIONS[row.type].map((s) => s.key)}
                      style={{ flex: 1 }}
                      value={row.key}
                      onChange={(next) => updateRow(index, { key: next })}
                    />
                    <Autocomplete
                      size="xs"
                      placeholder="checkout"
                      data={LABEL_VALUE_SUGGESTIONS[row.key] ?? []}
                      style={{ flex: 1 }}
                      value={row.value}
                      onChange={(next) => updateRow(index, { value: next })}
                    />
                  </Group>
                </Stack>
                <ActionIcon
                  color="red"
                  variant="subtle"
                  size="sm"
                  title="Remove label"
                  onClick={() => onChange(rows.filter((_, i) => i !== index))}
                >
                  <TrashIcon />
                </ActionIcon>
              </Group>
            ))}
          </Stack>
        </ScrollArea.Autosize>

        <Group justify="space-between">
          <Button
            variant="subtle"
            size="xs"
            leftSection={<PlusIcon />}
            onClick={() => onChange([...rows, { type: LabelType.LabelTypeIdentifying, key: '', value: '' }])}
          >
            Add label
          </Button>
          <Button size="xs" loading={loading} leftSection={<MagnifyingGlassIcon />} onClick={onMatch}>
            Find route
          </Button>
        </Group>

        {result && (
          result.matched ? (
            <Stack gap={4}>
              <Group gap={4}>
                {result.path.map((name, index) => (
                  <Badge key={`${name}-${index}`} variant="light" size="xs">{name || '(unnamed)'}</Badge>
                ))}
              </Group>
              <Text size="xs">→ {result.configRef || 'no config'}</Text>
            </Stack>
          ) : (
            <Alert color="yellow" p="xs" fz="xs">
              No route matches, these labels get the default config{result.configRef ? ` "${result.configRef}"` : ''}.
            </Alert>
          )
        )}
      </Stack>
    </Paper>
  );
}
