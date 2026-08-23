import { ActionIcon, Badge, Button, Group, Paper, Select, Stack, Text, TextInput } from '@mantine/core';
import { PlusIcon, TrashIcon } from '@radix-ui/react-icons';

import { LabelFilterEditor } from './LabelFilterEditor';
import { emptyLabelFilterValues, emptyRouteValues, type RouteValues } from './router';

interface RouteEditorProps {
  value: RouteValues;
  collectorConfigs: string[];
  defNames: string[];
  onChange: (value: RouteValues) => void;
  onRemove?: () => void;
  isRoot?: boolean;
  depth?: number;
}

export function RouteEditor({
  value,
  collectorConfigs,
  defNames,
  onChange,
  onRemove,
  isRoot = false,
  depth = 0,
}: RouteEditorProps) {
  const usesDef = value.use !== '';

  const addChild = () =>
    onChange({ ...value, routes: [...value.routes, emptyRouteValues()] });

  return (
    <Paper withBorder p="sm" ml={depth === 0 ? 0 : 'md'}>
      <Stack gap="sm">
        <Group justify="space-between" align="flex-end" wrap="nowrap">
          <Group gap="xs" align="flex-end" style={{ flex: 1 }} wrap="wrap">
            <TextInput
              label="Route name"
              placeholder="production"
              value={value.name}
              onChange={(event) => onChange({ ...value, name: event.currentTarget.value })}
              style={{ flex: 1, minWidth: 160 }}
            />
            <Select
              label="Collector config"
              placeholder="Select a collector config"
              data={collectorConfigs}
              searchable
              clearable
              disabled={usesDef}
              value={value.configRef || null}
              onChange={(next) => onChange({ ...value, configRef: next ?? '' })}
              style={{ flex: 1, minWidth: 200 }}
            />
            <Select
              label="Use definition"
              placeholder="None"
              data={defNames}
              searchable
              clearable
              value={value.use || null}
              onChange={(next) => onChange({ ...value, use: next ?? '' })}
              style={{ flex: 1, minWidth: 160 }}
            />
          </Group>
          {isRoot ? (
            <Badge variant="light">root</Badge>
          ) : (
            <ActionIcon color="red" variant="subtle" size="lg" title="Remove route" onClick={onRemove}>
              <TrashIcon />
            </ActionIcon>
          )}
        </Group>

        {usesDef && (
          <Text size="xs" c="dimmed">
            Filters, config and nested routes come from definition "{value.use}".
          </Text>
        )}

        {!usesDef && (
          <>
            <Group justify="space-between">
              <Text fw={500} size="sm">Label filters</Text>
              <Button
                variant="light"
                size="xs"
                leftSection={<PlusIcon />}
                onClick={() => onChange({ ...value, filters: [...value.filters, emptyLabelFilterValues()] })}
              >
                Add filter
              </Button>
            </Group>

            {value.filters.length === 0 && (
              <Text size="xs" c="dimmed">No filters, this route matches every collector.</Text>
            )}

            {value.filters.map((filter, index) => (
              <LabelFilterEditor
                key={index}
                value={filter}
                onChange={(next) =>
                  onChange({ ...value, filters: value.filters.map((f, i) => (i === index ? next : f)) })
                }
                onRemove={() => onChange({ ...value, filters: value.filters.filter((_, i) => i !== index) })}
              />
            ))}

            <Group justify="space-between">
              <Text fw={500} size="sm">Nested routes</Text>
              <Button variant="light" size="xs" leftSection={<PlusIcon />} onClick={addChild}>
                Add route
              </Button>
            </Group>

            {value.routes.map((child, index) => (
              <RouteEditor
                key={index}
                value={child}
                collectorConfigs={collectorConfigs}
                defNames={defNames}
                depth={depth + 1}
                onChange={(next) =>
                  onChange({ ...value, routes: value.routes.map((r, i) => (i === index ? next : r)) })
                }
                onRemove={() => onChange({ ...value, routes: value.routes.filter((_, i) => i !== index) })}
              />
            ))}
          </>
        )}
      </Stack>
    </Paper>
  );
}
