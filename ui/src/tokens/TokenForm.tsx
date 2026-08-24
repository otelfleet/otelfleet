import { ActionIcon, Button, Group, NumberInput, Paper, Stack, Text, TextInput } from '@mantine/core';
import { useForm } from '@mantine/form';
import { PlusIcon, TrashIcon } from '@radix-ui/react-icons';

import { emptyTokenLabel, type TokenValues } from './token';

interface TokenFormProps {
  initialValues: TokenValues;
  onSubmit: (values: TokenValues) => void;
  onCancel: () => void;
}

export function TokenForm({ initialValues, onSubmit, onCancel }: TokenFormProps) {
  const form = useForm<TokenValues>({
    mode: 'controlled',
    initialValues,
    validate: {
      ttlSeconds: (value) => (value > 0 ? null : 'TTL must be greater than zero'),
      labels: {
        key: (value) => (value.trim() === '' ? 'Label key is required' : null),
      },
    },
  });

  return (
    <form onSubmit={form.onSubmit(onSubmit)}>
      <Stack gap="md">
        <NumberInput
          withAsterisk
          label="TTL"
          description="Seconds before the token expires"
          min={1}
          w={240}
          {...form.getInputProps('ttlSeconds')}
        />

        <Stack gap="sm">
          <Group justify="space-between">
            <Stack gap={0}>
              <Text fw={500}>Labels</Text>
              <Text size="sm" c="dimmed">
                Applied to agents that bootstrap with this token, and matched by config filters
              </Text>
            </Stack>
            <Button
              variant="light"
              size="xs"
              leftSection={<PlusIcon />}
              onClick={() => form.insertListItem('labels', emptyTokenLabel())}
            >
              Add label
            </Button>
          </Group>

          {form.getValues().labels.length === 0 ? (
            <Text size="sm" c="dimmed">No labels defined</Text>
          ) : (
            <Paper withBorder p="sm">
              <Stack gap="xs">
                {form.getValues().labels.map((_label, index) => (
                  <Group key={index} align="flex-end" gap="xs" wrap="nowrap">
                    <TextInput
                      label="Key"
                      placeholder="environment"
                      style={{ flex: 1 }}
                      {...form.getInputProps(`labels.${index}.key`)}
                    />
                    <TextInput
                      label="Value"
                      placeholder="production"
                      style={{ flex: 1 }}
                      {...form.getInputProps(`labels.${index}.value`)}
                    />
                    <ActionIcon
                      color="red"
                      variant="subtle"
                      size="lg"
                      title="Remove label"
                      onClick={() => form.removeListItem('labels', index)}
                    >
                      <TrashIcon />
                    </ActionIcon>
                  </Group>
                ))}
              </Stack>
            </Paper>
          )}
        </Stack>

        <Group justify="flex-end">
          <Button variant="default" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="submit">Create token</Button>
        </Group>
      </Stack>
    </form>
  );
}
