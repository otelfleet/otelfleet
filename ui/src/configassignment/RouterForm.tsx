import { Button, Group, Select, Stack, Text } from '@mantine/core';

import { RouteEditor } from './RouteEditor';
import { emptyRouteValues, type RouterValues } from './router';

interface RouterFormProps {
  value: RouterValues;
  collectorConfigs: string[];
  onChange: (value: RouterValues) => void;
}

export function RouterForm({ value, collectorConfigs, onChange }: RouterFormProps) {
  const defNames = value.defs.map((def) => def.name).filter((name) => name !== '');

  return (
    <Stack gap="md">
      <Select
        label="Default collector config"
        description="Delivered to collectors that no route matches"
        placeholder="Select a collector config"
        data={collectorConfigs}
        searchable
        clearable
        value={value.configRef || null}
        onChange={(next) => onChange({ ...value, configRef: next ?? '' })}
      />

      <Text fw={500}>Routes</Text>
      <RouteEditor
        isRoot
        value={value.root}
        collectorConfigs={collectorConfigs}
        defNames={defNames}
        onChange={(root) => onChange({ ...value, root })}
      />

      <Group justify="space-between">
        <Text fw={500}>Reusable route definitions</Text>
        <Button
          variant="light"
          size="xs"
          onClick={() => onChange({ ...value, defs: [...value.defs, emptyRouteValues()] })}
        >
          Add definition
        </Button>
      </Group>

      {value.defs.length === 0 && (
        <Text size="xs" c="dimmed">
          No definitions. Definitions are route subtrees that other routes reference by name.
        </Text>
      )}

      {value.defs.map((def, index) => (
        <RouteEditor
          key={index}
          value={def}
          collectorConfigs={collectorConfigs}
          defNames={defNames.filter((name) => name !== def.name)}
          onChange={(next) => onChange({ ...value, defs: value.defs.map((d, i) => (i === index ? next : d)) })}
          onRemove={() => onChange({ ...value, defs: value.defs.filter((_, i) => i !== index) })}
        />
      ))}
    </Stack>
  );
}
