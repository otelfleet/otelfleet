import { useState, useEffect, useCallback, useMemo } from 'react';
import { Link } from '@tanstack/react-router';
import { Group, Button, ActionIcon, Modal, Text } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { TrashIcon, Pencil1Icon, CheckCircledIcon } from '@radix-ui/react-icons';

import { Table, type ColumnConfig } from '../components/Table';
import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { ResourceService } from '../gen/api/pkg/api/resources/v1alpha1/resources_pb';
import type { EntityType } from './entityTypes';

interface EntityRow {
  key: string;
}

export function ResourceListPage({ entityType }: { entityType: EntityType }) {
  const client = useClient(ResourceService);

  const [rows, setRows] = useState<EntityRow[]>([]);
  const [deleteModalOpened, { open: openDeleteModal, close: closeDeleteModal }] = useDisclosure(false);
  const [keyToDelete, setKeyToDelete] = useState<string | null>(null);

  const listEntities = useCallback(async () => {
    try {
      const response = await client.listEntity({ typeUrl: entityType.typeUrl });
      setRows(response.entities.map((e) => ({ key: e.key })));
    } catch (error) {
      notifyGRPCError(`Failed to list ${entityType.label} entities`, error);
    }
  }, [client, entityType]);

  const handleDelete = useCallback(async () => {
    if (!keyToDelete) return;
    try {
      await client.deleteEntity({ typeUrl: entityType.typeUrl, key: keyToDelete });
      notifications.show({
        title: `${entityType.label} deleted`,
        message: `Removed "${keyToDelete}"`,
        icon: <CheckCircledIcon />,
      });
      listEntities();
    } catch (error) {
      notifyGRPCError(`Failed to delete ${entityType.label}`, error);
    } finally {
      closeDeleteModal();
      setKeyToDelete(null);
    }
  }, [client, entityType, keyToDelete, listEntities, closeDeleteModal]);

  const confirmDelete = useCallback((key: string) => {
    setKeyToDelete(key);
    openDeleteModal();
  }, [openDeleteModal]);

  const columns = useMemo<ColumnConfig<EntityRow>[]>(() => [
    { key: 'key', label: 'Name', visible: true },
    {
      key: 'actions',
      label: 'Actions',
      visible: true,
      render: (_value: unknown, row: EntityRow) => (
        <Group gap="xs" justify="center">
          <Link to="/resources/$type/editor" params={{ type: entityType.slug }} search={{ key: row.key }}>
            <ActionIcon variant="subtle" size="lg" title={`Edit ${entityType.label}`}>
              <Pencil1Icon width={18} height={18} />
            </ActionIcon>
          </Link>
          <ActionIcon color="red" variant="subtle" size="lg" onClick={() => confirmDelete(row.key)} title={`Delete ${entityType.label}`}>
            <TrashIcon width={18} height={18} />
          </ActionIcon>
        </Group>
      ),
    },
  ], [entityType, confirmDelete]);

  useEffect(() => {
    listEntities();
  }, [listEntities]);

  return (
    <>
      <Group style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
        <Link to="/resources/$type/editor" params={{ type: entityType.slug }} style={{ display: 'inline-block' }}>
          <Button leftSection={<entityType.icon />}>New {entityType.label}</Button>
        </Link>
      </Group>
      <Table<EntityRow>
        title={entityType.description}
        data={rows}
        columns={columns}
        rowKey="key"
      />

      <Modal opened={deleteModalOpened} onClose={closeDeleteModal} title="Confirm Delete">
        <Text>Are you sure you want to delete "{keyToDelete}"? This action cannot be undone.</Text>
        <Group justify="flex-end" mt="md">
          <Button variant="default" onClick={closeDeleteModal}>Cancel</Button>
          <Button color="red" onClick={handleDelete}>Delete</Button>
        </Group>
      </Modal>
    </>
  );
}
