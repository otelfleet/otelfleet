import {
  ActionIcon,
  Badge,
  Box,
  Button,
  Collapse,
  Group,
  Paper,
  Select,
  Stack,
  Text,
  TextInput,
} from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { ChevronDownIcon, ChevronRightIcon, PlusIcon, TrashIcon } from '@radix-ui/react-icons';

import { LabelFilterEditor } from './LabelFilterEditor';
import { emptyLabelFilterValues, emptyRouteValues, type RouteValues } from './router';

const DEPTH_COLORS = ['blue', 'grape', 'teal', 'orange', 'pink'];

function depthColor(depth: number) {
  return DEPTH_COLORS[depth % DEPTH_COLORS.length];
}

interface RouteEditorProps {
  value: RouteValues;
  collectorConfigs: string[];
  defNames: string[];
  onChange: (value: RouteValues) => void;
  onRemove?: () => void;
  isRoot?: boolean;
  badgeLabel?: string;
  depth?: number;
}

export function RouteEditor({
  value,
  collectorConfigs,
  defNames,
  onChange,
  onRemove,
  isRoot = false,
  badgeLabel,
  depth = 0,
}: RouteEditorProps) {
  const [opened, { toggle }] = useDisclosure(true);
  const usesDef = value.use !== '';
  const color = depthColor(depth);

  const addChild = () =>
    onChange({ ...value, routes: [...value.routes, emptyRouteValues()] });

  return (
    <Paper
      withBorder
      p="sm"
      radius="sm"
      bg={depth % 2 === 0 ? 'var(--mantine-color-body)' : 'var(--mantine-color-default)'}
      style={{ borderLeft: `4px solid var(--mantine-color-${color}-filled)` }}
    >
      <Stack gap="sm">
        <Group justify="space-between" align="center" wrap="nowrap">
          <Group gap="xs" wrap="nowrap">
            <ActionIcon variant="subtle" color="gray" size="sm" onClick={toggle} title={opened ? 'Collapse' : 'Expand'}>
              {opened ? <ChevronDownIcon /> : <ChevronRightIcon />}
            </ActionIcon>
            <Badge variant="light" color={color} size="sm">
              {badgeLabel ?? (isRoot ? 'default' : `depth ${depth}`)}
            </Badge>
            <Text fw={600} size="sm" c={value.name ? undefined : 'dimmed'}>
              {value.name || 'unnamed route'}
            </Text>
            {!opened && (
              <Text size="xs" c="dimmed">
                {value.filters.length} filters · {value.routes.length} nested
              </Text>
            )}
          </Group>
          {onRemove && (
            <ActionIcon color="red" variant="subtle" size="lg" title="Remove route" onClick={onRemove}>
              <TrashIcon />
            </ActionIcon>
          )}
        </Group>

        <Collapse in={opened}>
          <Stack gap="sm">
            <Group gap="xs" align="flex-end" wrap="wrap">
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

            {usesDef && (
              <Text size="xs" c="dimmed">
                Filters, config and nested routes come from definition "{value.use}".
              </Text>
            )}

            {!usesDef && (
              <>
                <Group justify="space-between">
                  <Group gap="xs">
                    <Text fw={500} size="sm">Label filters</Text>
                    <Badge variant="default" size="xs">{value.filters.length}</Badge>
                  </Group>
                  <Button
                    variant="light"
                    color={color}
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

                <Stack gap="xs">
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
                </Stack>

                <Group justify="space-between">
                  <Group gap="xs">
                    <Text fw={500} size="sm">Nested routes</Text>
                    <Badge variant="default" size="xs">{value.routes.length}</Badge>
                  </Group>
                  <Button variant="light" color={depthColor(depth + 1)} size="xs" leftSection={<PlusIcon />} onClick={addChild}>
                    Add route
                  </Button>
                </Group>

                {value.routes.length > 0 && (
                  <Box
                    pl="md"
                    style={{ borderLeft: `1px dashed var(--mantine-color-${depthColor(depth + 1)}-outline)` }}
                  >
                    <Stack gap="xs">
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
                    </Stack>
                  </Box>
                )}
              </>
            )}
          </Stack>
        </Collapse>
      </Stack>
    </Paper>
  );
}
