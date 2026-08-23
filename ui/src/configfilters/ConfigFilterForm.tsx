import {
  ActionIcon,
  Autocomplete,
  Button,
  Group,
  Paper,
  Select,
  Stack,
  Switch,
  Text,
  TextInput,
  ThemeIcon,
  Tooltip,
  type ComboboxStringItem,
} from '@mantine/core';
import { useForm } from '@mantine/form';
import { InfoCircledIcon, PlusIcon, TrashIcon } from '@radix-ui/react-icons';

import {
  LABEL_KEY_SUGGESTIONS,
  LABEL_SCOPE_DESCRIPTIONS,
  LABEL_SCOPE_OPTIONS,
  LABEL_VALUE_SUGGESTIONS,
  MATCH_TYPE_OPTIONS,
  emptyLabelFilterValues,
  emptyLabelRow,
  type ConfigFilterValues,
  type LabelScope,
} from './configFilter';
import { MatchType } from '../gen/api/pkg/api/resources/v1alpha1/resources_pb';

function renderLabelKeyOption(scope: LabelScope) {
  return ({ option }: { option: ComboboxStringItem }) => {
    const description = LABEL_KEY_SUGGESTIONS[scope].find((s) => s.key === option.value)?.description;
    return (
      <Stack gap={0}>
        <Text size="sm">{option.value}</Text>
        {description && <Text size="xs" c="dimmed">{description}</Text>}
      </Stack>
    );
  };
}

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

  const hasFilters = form.getValues().filters.length > 0;
  const addFilter = () => form.insertListItem('filters', emptyLabelFilterValues());

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
            {hasFilters && (
              <Button variant="light" size="xs" leftSection={<PlusIcon />} onClick={addFilter}>
                Add filter
              </Button>
            )}
          </Group>

          {!hasFilters && (
            <Paper withBorder p="xl">
              <Stack align="center" gap="xs">
                <Text size="sm" c="dimmed">
                  No label filters, create one to match on collector runtime information or user-defined labels.
                </Text>
                <Button variant="light" size="md" leftSection={<PlusIcon />} onClick={addFilter}>
                  Add filter
                </Button>
              </Stack>
            </Paper>
          )}

          {form.getValues().filters.map((filter, filterIndex) => (
            <Paper key={filterIndex} withBorder p="sm">
              <Stack gap="xs">
                <Group justify="space-between">
                  <Group gap="xs" align="flex-end">
                    <Select
                      label="Scope"
                      data={LABEL_SCOPE_OPTIONS}
                      w={220}
                      {...form.getInputProps(`filters.${filterIndex}.scope`)}
                      onChange={(value) =>
                        form.setFieldValue(`filters.${filterIndex}.scope`, (value ?? 'identifying') as LabelScope)
                      }
                    />
                    <Tooltip label={LABEL_SCOPE_DESCRIPTIONS[filter.scope]} withArrow>
                      <ThemeIcon variant="subtle" color="gray" size="lg">
                        <InfoCircledIcon />
                      </ThemeIcon>
                    </Tooltip>
                  </Group>
                  <ActionIcon
                    color="red"
                    variant="subtle"
                    title="Remove filter"
                    onClick={() => form.removeListItem('filters', filterIndex)}
                  >
                    <TrashIcon />
                  </ActionIcon>
                </Group>

                {filter.labels.map((label, labelIndex) => (
                  <Group key={labelIndex} align="flex-end" gap="xs" wrap="nowrap">
                    <Autocomplete
                      label="Label"
                      placeholder="service.name"
                      data={LABEL_KEY_SUGGESTIONS[filter.scope].map((s) => s.key)}
                      renderOption={renderLabelKeyOption(filter.scope)}
                      style={{ flex: 1 }}
                      {...form.getInputProps(`filters.${filterIndex}.labels.${labelIndex}.key`)}
                    />
                    <Select
                      label="Operator"
                      data={MATCH_TYPE_OPTIONS}
                      w={90}
                      value={String(label.type)}
                      onChange={(value) =>
                        form.setFieldValue(
                          `filters.${filterIndex}.labels.${labelIndex}.type`,
                          Number(value) as MatchType,
                        )
                      }
                    />
                    <Autocomplete
                      label="Value"
                      placeholder="checkout"
                      data={LABEL_VALUE_SUGGESTIONS[label.key] ?? []}
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
                  onClick={() => form.insertListItem(`filters.${filterIndex}.labels`, emptyLabelRow())}
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
