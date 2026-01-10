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
// import { IconMenu2 } from '@mantine/icons';
import { useState, useMemo } from 'react';
import {Menu as MenuIcon} from 'react-feather'
type Person = {
  firstName: string;
  lastName: string;
  age: number;
  visits: number;
  status: string;
  progress: number;
};

export type ColumnConfig<T> = {
  key: keyof T;
  label: string;
  visible?: boolean;
  render?: (value: any, row: T) => React.ReactNode;
};

interface DynamicTableProps<T> {
  data: T[];
  columns: ColumnConfig<T>[];
  title?: string;
}

export const Table = <T,>({
  data,
  columns,
  title,
}: DynamicTableProps<T>): React.ReactElement => {
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
            <tr key={idx}>
              {activeColumns.map((col) => (
                <td key={String(col.key)}>
                  {col.render
                    ? col.render(row[col.key], row)
                    : String(row[col.key])}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </MantineTable>
    </Paper>
  );
};

// Example usage
const defaultData: Person[] = [
  {
    firstName: 'tanner',
    lastName: 'linsley',
    age: 24,
    visits: 100,
    status: 'In Relationship',
    progress: 50,
  },
  {
    firstName: 'tandy',
    lastName: 'miller',
    age: 40,
    visits: 40,
    status: 'Single',
    progress: 80,
  },
  {
    firstName: 'joe',
    lastName: 'dirte',
    age: 45,
    visits: 20,
    status: 'Complicated',
    progress: 10,
  },
];

const personColumns: ColumnConfig<Person>[] = [
  { key: 'firstName', label: 'First Name', visible: true },
  { key: 'lastName', label: 'Last Name', visible: true },
  { key: 'age', label: 'Age', visible: true },
  { key: 'visits', label: 'Visits', visible: true },
  { key: 'status', label: 'Status', visible: true },
  { key: 'progress', label: 'Progress (%)', visible: false },
];

export const TableExample = () => (
  <Table<Person> title="Persons" data={defaultData} columns={personColumns} />
);