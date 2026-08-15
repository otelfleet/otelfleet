import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { ActionIcon, Badge, Button, Group, Modal, Text } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { CheckCircledIcon, MixerHorizontalIcon, Pencil1Icon, TrashIcon } from '@radix-ui/react-icons';

import { Table, type ColumnConfig } from '../components/Table';
import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { ResourceService, type ConfigFilter } from '../gen/api/pkg/api/resources/v1alpha1/resources_pb';
import { CONFIG_FILTER_TYPE_URL, unpackConfigFilter } from './configFilter';

interface ConfigFilterRow {
  key: string;
  filter?: ConfigFilter;
}

export function ConfigFilterPage() {
  const client = useClient(ResourceService);

  const [rows, setRows] = useState<ConfigFilterRow[]>([]);
  const [keyToDelete, setKeyToDelete] = useState<string | null>(null);
  const [deleteOpened, { open: openDelete, close: closeDelete }] = useDisclosure(false);

  const listFilters = useCallback(async () => {
    try {
      const response = await client.listEntity({ typeUrl: CONFIG_FILTER_TYPE_URL });
      setRows(response.entities.map((e) => ({ key: e.key, filter: unpackConfigFilter(e.obj) })));
    } catch (error) {
      notifyGRPCError('Failed to list config assignments', error);
    }
  }, [client]);

  useEffect(() => {
    listFilters();
  }, [listFilters]);

  const handleDelete = useCallback(async () => {
    if (!keyToDelete) return;
    try {
      await client.deleteEntity({ typeUrl: CONFIG_FILTER_TYPE_URL, key: keyToDelete });
      notifications.show({
        title: 'Config assignment deleted',
        message: `Removed "${keyToDelete}"`,
        icon: <CheckCircledIcon />,
      });
      listFilters();
    } catch (error) {
      notifyGRPCError('Failed to delete config assignment', error);
    } finally {
      closeDelete();
      setKeyToDelete(null);
    }
  }, [client, keyToDelete, listFilters, closeDelete]);

  const confirmDelete = useCallback((key: string) => {
    setKeyToDelete(key);
    openDelete();
  }, [openDelete]);

  const columns = useMemo<ColumnConfig<ConfigFilterRow>[]>(() => [
    { key: 'key', label: 'Name', visible: true },
    {
      key: 'configRef',
      label: 'Collector config',
      visible: true,
      render: (_value: unknown, row: ConfigFilterRow) => (
        <Text size="sm">{row.filter?.collectorConfig?.configRef || '—'}</Text>
      ),
    },
    {
      key: 'default',
      label: 'Default',
      visible: true,
      render: (_value: unknown, row: ConfigFilterRow) =>
        row.filter?.default ? <Badge color="blue">default</Badge> : null,
    },
    {
      key: 'actions',
      label: 'Actions',
      visible: true,
      render: (_value: unknown, row: ConfigFilterRow) => (
        <Group gap="xs" justify="center">
          <Link to="/configfilter/editor" search={{ key: row.key }}>
            <ActionIcon variant="subtle" size="lg" title="Edit assignment">
              <Pencil1Icon width={18} height={18} />
            </ActionIcon>
          </Link>
          <ActionIcon
            color="red"
            variant="subtle"
            size="lg"
            title="Delete assignment"
            onClick={() => confirmDelete(row.key)}
          >
            <TrashIcon width={18} height={18} />
          </ActionIcon>
        </Group>
      ),
    },
  ], [confirmDelete]);

  return (
    <>
      <Group style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
        <Link to="/configfilter/editor" style={{ display: 'inline-block' }}>
          <Button leftSection={<MixerHorizontalIcon />}>New</Button>
        </Link>
      </Group>

      <Table<ConfigFilterRow>
        title="Config Filter"
        data={rows}
        columns={columns}
        rowKey="key"
      />

      <Modal opened={deleteOpened} onClose={closeDelete} title="Confirm Delete">
        <Text>Are you sure you want to delete "{keyToDelete}"? This action cannot be undone.</Text>
        <Group justify="flex-end" mt="md">
          <Button variant="default" onClick={closeDelete}>Cancel</Button>
          <Button color="red" onClick={handleDelete}>Delete</Button>
        </Group>
      </Modal>
    </>
  );
}
