import {
  Table as MantineTable,
  Checkbox,
  Menu,
  ActionIcon,
  Paper,
  Group,
  Stack,
  Text,
  Box,
} from '@mantine/core';
import { useState, useMemo } from 'react';
import { Menu as MenuIcon } from 'react-feather';

export type ColumnConfig<T> = {
  key: keyof T | string;
  label: string;
  visible?: boolean;
  render?: (value: any, row: T) => React.ReactNode;
};

interface DynamicTableProps<T> {
  data: T[];
  columns: ColumnConfig<T>[];
  title?: string;
  rowKey?: keyof T | ((row: T, index: number) => string | number);
}

export const Table = <T extends object>({
  data,
  columns,
  title,
  rowKey,
}: DynamicTableProps<T>): React.ReactElement => {
  const getRowKey = (row: T, index: number): string | number => {
    if (!rowKey) return index;
    if (typeof rowKey === 'function') return rowKey(row, index);
    return String(row[rowKey]);
  };
  const [visibleColumns, setVisibleColumns] = useState<Set<string>>(
    new Set(
      columns
        .filter((col) => col.visible !== false)
        .map((col) => String(col.key))
    )
  );

  const activeColumns = useMemo(
    () => columns.filter((col) => visibleColumns.has(String(col.key))),
    [columns, visibleColumns]
  );

  const handleColumnToggle = (columnKey: string) => {
    const next = new Set(visibleColumns);
    next.has(columnKey) ? next.delete(columnKey) : next.add(columnKey);
    setVisibleColumns(next);
  };

  return (
    <Paper withBorder radius="md">
      <Group
        px="sm"
        py="xs"
        justify="space-between"
        style={{ borderBottom: '1px solid var(--mantine-color-gray-3)' }}
      >
        <Menu shadow="md" width={200}>
          <Menu.Target>
            <ActionIcon variant="subtle">
              <MenuIcon></MenuIcon>
            </ActionIcon>
          </Menu.Target>
          <Menu.Dropdown>
            <Stack gap="xs" p="xs">
              {columns.map((col) => (
                <Checkbox
                  key={String(col.key)}
                  label={col.label}
                  checked={visibleColumns.has(String(col.key))}
                  onChange={() => handleColumnToggle(String(col.key))}
                />
              ))}
            </Stack>
          </Menu.Dropdown>
        </Menu>

        <Box style={{ flex: 1, textAlign: 'center' }}>
          <Text fw={600}>{title}</Text>
        </Box>

        {/* empty spacer to match layout */}
        <Box style={{ width: 28 }} />
      </Group>

      <MantineTable striped highlightOnHover ta="center">
        <thead>
          <tr>
            {activeColumns.map((col) => (
              <th key={String(col.key)}>
                <Text fw={600}>{col.label}</Text>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, idx) => (
            <tr key={getRowKey(row, idx)}>
              {activeColumns.map((col) => {
                const value = col.key in row ? row[col.key as keyof T] : undefined;
                return (
                  <td key={String(col.key)}>
                    {col.render
                      ? col.render(value, row)
                      : String(value ?? '')}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </MantineTable>
    </Paper>
  );
};