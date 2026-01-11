import {
  Table,
  type ColumnConfig
} from '../components/Table'

import { useClient } from '../api';
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import { useState, useEffect, useCallback, useMemo } from 'react';
import type { ConfigReference } from '../gen/api/pkg/api/config/v1alpha1/config_pb'
import { Link } from '@tanstack/react-router'
import { Group, Button, ActionIcon, Modal, Text } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { notifyGRPCError } from '../api/notifications';
import { CheckCircledIcon, TrashIcon, Pencil1Icon } from '@radix-ui/react-icons';

export const ConfigPage = () => {
  const client = useClient(ConfigService)

  const [configState, setConfigState] = useState<ConfigReference[]>([])
  const [deleteModalOpened, { open: openDeleteModal, close: closeDeleteModal }] = useDisclosure(false)
  const [configToDelete, setConfigToDelete] = useState<string | null>(null)

  const handleListConfigs = useCallback(async () => {
    try {
      const response = await client.listConfigs({})
      setConfigState(response.configs)
    } catch (error) {
      notifyGRPCError("Failed to list configs", error)
    }
  }, [client])

  const handleDeleteConfig = useCallback(async () => {
    if (!configToDelete) return
    try {
      await client.deleteConfig({ id: configToDelete })
      notifications.show({
        title: "Config deleted",
        message: "Config has been removed",
        icon: <CheckCircledIcon />,
      })
      handleListConfigs()
    } catch (error) {
      notifyGRPCError("Failed to delete config", error)
    } finally {
      closeDeleteModal()
      setConfigToDelete(null)
    }
  }, [client, handleListConfigs, configToDelete, closeDeleteModal])

  const confirmDelete = useCallback((configId: string) => {
    setConfigToDelete(configId)
    openDeleteModal()
  }, [openDeleteModal])

  const configColumns = useMemo<ColumnConfig<ConfigReference>[]>(() => [
    { key: 'id', label: 'Name', visible: true },
    {
      key: 'actions',
      label: 'Actions',
      visible: true,
      render: (_value: unknown, row: ConfigReference) => (
        <Group gap="xs" justify="center">
          <Link to="/editor" search={{ id: row.id }}>
            <ActionIcon variant="subtle" size="lg">
              <Pencil1Icon width={18} height={18} />
            </ActionIcon>
          </Link>
          <ActionIcon color="red" variant="subtle" size="lg" onClick={() => confirmDelete(row.id)}>
            <TrashIcon width={18} height={18} />
          </ActionIcon>
        </Group>
      )
    },
  ], [confirmDelete])

  useEffect(() => {
    handleListConfigs()
  }, [handleListConfigs])

  return (
    <>
      <Group style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
        <Link to="/editor" style={{ display: 'inline-block' }}>
          <Button>
            New Config
          </Button>
        </Link>
      </Group>
      <Table<ConfigReference>
        title="OpenTelemetry Collector configurations"
        data={configState}
        columns={configColumns}
        rowKey="id"
      />

      <Modal opened={deleteModalOpened} onClose={closeDeleteModal} title="Confirm Delete">
        <Text>Are you sure you want to delete this config? This action cannot be undone.</Text>
        <Group justify="flex-end" mt="md">
          <Button variant="default" onClick={closeDeleteModal}>Cancel</Button>
          <Button color="red" onClick={handleDeleteConfig}>Delete</Button>
        </Group>
      </Modal>
    </>
  )
}
