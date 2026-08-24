import {
  ActionIcon,
  Autocomplete,
  Button,
  Group,
  Paper,
  Select,
  Stack,
  Text,
  ThemeIcon,
  Tooltip,
  type ComboboxStringItem,
} from '@mantine/core';
import { InfoCircledIcon, PlusIcon, TrashIcon } from '@radix-ui/react-icons';

import { LabelType, MatchType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import {
  LABEL_KEY_SUGGESTIONS,
  LABEL_TYPE_DESCRIPTIONS,
  LABEL_TYPE_OPTIONS,
  LABEL_VALUE_SUGGESTIONS,
  MATCH_TYPE_OPTIONS,
  emptyLabelRow,
  type LabelFilterValues,
  type LabelRow,
} from './router';

function renderLabelKeyOption(type: LabelType) {
  return ({ option }: { option: ComboboxStringItem }) => {
    const description = LABEL_KEY_SUGGESTIONS[type].find((s) => s.key === option.value)?.description;
    return (
      <Stack gap={0}>
        <Text size="sm">{option.value}</Text>
        {description && <Text size="xs" c="dimmed">{description}</Text>}
      </Stack>
    );
  };
}

interface LabelFilterEditorProps {
  value: LabelFilterValues;
  onChange: (value: LabelFilterValues) => void;
  onRemove: () => void;
}

export function LabelFilterEditor({ value, onChange, onRemove }: LabelFilterEditorProps) {
  const updateLabel = (index: number, row: Partial<LabelRow>) =>
    onChange({
      ...value,
      labels: value.labels.map((label, i) => (i === index ? { ...label, ...row } : label)),
    });

  return (
    <Paper withBorder p="sm">
      <Stack gap="xs">
        <Group justify="space-between">
          <Group gap="xs" align="flex-end">
            <Select
              label="Scope"
              data={LABEL_TYPE_OPTIONS}
              w={220}
              value={String(value.type)}
              onChange={(next) => onChange({ ...value, type: Number(next ?? LabelType.LabelTypeIdentifying) })}
            />
            <Tooltip label={LABEL_TYPE_DESCRIPTIONS[value.type]} withArrow>
              <ThemeIcon variant="subtle" color="gray" size="lg">
                <InfoCircledIcon />
              </ThemeIcon>
            </Tooltip>
          </Group>
          <ActionIcon color="red" variant="subtle" title="Remove filter" onClick={onRemove}>
            <TrashIcon />
          </ActionIcon>
        </Group>

        {value.labels.map((label, index) => (
          <Group key={index} align="flex-end" gap="xs" wrap="nowrap">
            <Autocomplete
              label="Label"
              placeholder="service.name"
              data={LABEL_KEY_SUGGESTIONS[value.type].map((s) => s.key)}
              renderOption={renderLabelKeyOption(value.type)}
              style={{ flex: 1 }}
              value={label.key}
              onChange={(next) => updateLabel(index, { key: next })}
            />
            <Select
              label="Operator"
              data={MATCH_TYPE_OPTIONS}
              w={90}
              value={String(label.matchType)}
              onChange={(next) => updateLabel(index, { matchType: Number(next ?? MatchType.EQ) })}
            />
            <Autocomplete
              label="Value"
              placeholder="checkout"
              data={LABEL_VALUE_SUGGESTIONS[label.key] ?? []}
              style={{ flex: 1 }}
              value={label.value}
              onChange={(next) => updateLabel(index, { value: next })}
            />
            <ActionIcon
              color="red"
              variant="subtle"
              size="lg"
              title="Remove label"
              onClick={() => onChange({ ...value, labels: value.labels.filter((_, i) => i !== index) })}
            >
              <TrashIcon />
            </ActionIcon>
          </Group>
        ))}

        <Button
          variant="subtle"
          size="xs"
          leftSection={<PlusIcon />}
          onClick={() => onChange({ ...value, labels: [...value.labels, emptyLabelRow()] })}
        >
          Add label
        </Button>
      </Stack>
    </Paper>
  );
}
