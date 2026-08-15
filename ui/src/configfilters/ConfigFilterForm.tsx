import { Button, Group, Paper, Select, Stack, Switch, Text, TextInput, ActionIcon } from '@mantine/core';
import { useForm } from '@mantine/form';
import { PlusIcon, TrashIcon } from '@radix-ui/react-icons';

import {
  LABEL_SCOPE_OPTIONS,
  MATCH_TYPE_OPTIONS,
  emptyLabelFilterValues,
  type ConfigFilterValues,
  type LabelScope,
} from './configFilter';
import { MatchType } from '../gen/api/pkg/api/resources/v1alpha1/resources_pb';

interface ConfigFilterFormProps {
  initialValues: ConfigFilterValues;
  collectorConfigs: string[];
  editing: boolean;
  onSubmit: (values: ConfigFilterValues) => void;
  onCancel: () => void;
}

export function ConfigFilterForm({
  initialValues,
  collectorConfigs,
  editing,
  onSubmit,
  onCancel,
}: ConfigFilterFormProps) {
  const form = useForm<ConfigFilterValues>({
    mode: 'controlled',
    initialValues,
    validate: {
      name: (value) => (/^[a-zA-Z0-9][a-zA-Z0-9._-]*$/.test(value) ? null : 'Invalid name'),
      configRef: (value) => (value === '' ? 'Pick a collector config' : null),
    },
  });

  return (
    <form onSubmit={form.onSubmit(onSubmit)}>
      <Stack gap="md">
        <TextInput
          withAsterisk
          label="Name"
          placeholder="production-linux"
          disabled={editing}
          {...form.getInputProps('name')}
        />

        <Select
          withAsterisk
          label="Collector config"
          placeholder="Select a collector config"
          data={collectorConfigs}
          searchable
          {...form.getInputProps('configRef')}
        />

        <Group gap="xl">
          <Switch
            label="Default assignment"
            description="Used when no other filter matches"
            {...form.getInputProps('isDefault', { type: 'checkbox' })}
          />
          <Switch
            label="Requires approval"
            {...form.getInputProps('requiresApproval', { type: 'checkbox' })}
          />
        </Group>

        <Stack gap="sm">
          <Group justify="space-between">
            <Text fw={500}>Label filters</Text>
            <Button
              variant="light"
              size="xs"
              leftSection={<PlusIcon />}
              onClick={() => form.insertListItem('filters', emptyLabelFilterValues())}
            >
              Add filter
            </Button>
          </Group>

          {form.getValues().filters.map((filter, filterIndex) => (
            <Paper key={filterIndex} withBorder p="sm">
              <Stack gap="xs">
                <Group justify="space-between">
                  <Select
                    label="Match type"
                    data={MATCH_TYPE_OPTIONS}
                    value={String(filter.type)}
                    onChange={(value) =>
                      form.setFieldValue(`filters.${filterIndex}.type`, Number(value) as MatchType)
                    }
                    w={200}
                  />
                  <ActionIcon
                    color="red"
                    variant="subtle"
                    title="Remove filter"
                    onClick={() => form.removeListItem('filters', filterIndex)}
                  >
                    <TrashIcon />
                  </ActionIcon>
                </Group>

                {filter.labels.map((_label, labelIndex) => (
                  <Group key={labelIndex} align="flex-end" gap="xs" wrap="nowrap">
                    <Select
                      label="Scope"
                      data={LABEL_SCOPE_OPTIONS}
                      w={220}
                      {...form.getInputProps(`filters.${filterIndex}.labels.${labelIndex}.scope`)}
                      onChange={(value) =>
                        form.setFieldValue(
                          `filters.${filterIndex}.labels.${labelIndex}.scope`,
                          (value ?? 'identifying') as LabelScope,
                        )
                      }
                    />
                    <TextInput
                      label="Label"
                      placeholder="service.name"
                      style={{ flex: 1 }}
                      {...form.getInputProps(`filters.${filterIndex}.labels.${labelIndex}.key`)}
                    />
                    <TextInput
                      label="Value"
                      placeholder="checkout"
                      style={{ flex: 1 }}
                      {...form.getInputProps(`filters.${filterIndex}.labels.${labelIndex}.value`)}
                    />
                    <ActionIcon
                      color="red"
                      variant="subtle"
                      size="lg"
                      title="Remove label"
                      onClick={() => form.removeListItem(`filters.${filterIndex}.labels`, labelIndex)}
                    >
                      <TrashIcon />
                    </ActionIcon>
                  </Group>
                ))}

                <Button
                  variant="subtle"
                  size="xs"
                  leftSection={<PlusIcon />}
                  onClick={() =>
                    form.insertListItem(`filters.${filterIndex}.labels`, {
                      scope: 'identifying',
                      key: '',
                      value: '',
                    })
                  }
                >
                  Add label
                </Button>
              </Stack>
            </Paper>
          ))}
        </Stack>

        <Group justify="flex-end">
          <Button variant="default" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="submit">{editing ? 'Update assignment' : 'Create assignment'}</Button>
        </Group>
      </Stack>
    </form>
  );
}
