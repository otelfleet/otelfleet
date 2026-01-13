import {
  Table,
  type ColumnConfig
} from '../components/Table'
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { Box, Button, ActionIcon, Modal, Group, Text, TextInput, Select, Stack, Badge } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { useClient } from '../api';
import { TokenService, CreateTokenRequestSchema } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb';
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import type { ConfigReference } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import { useState, useEffect, useCallback, useMemo } from 'react';
import type { BootstrapToken } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb';
import { create } from '@bufbuild/protobuf';
import { CheckCircledIcon, TrashIcon, PlusIcon, Cross2Icon, EyeOpenIcon } from '@radix-ui/react-icons';
import { notifyGRPCError } from '../api/notifications';
import { useNavigate } from '@tanstack/react-router';

function timestampToDate(ts?: Timestamp | null): Date | null {
  if (!ts) return null;

  const rawSec = (ts as any).seconds ?? 0;
  const seconds =
    typeof rawSec === "number"
      ? rawSec
      : typeof rawSec === "string"
        ? parseInt(rawSec, 10)
        : typeof rawSec === "bigint"
          ? Number(rawSec)
          : typeof rawSec?.toNumber === "function"
            ? rawSec.toNumber()
            : Number(rawSec);

  const nanos = Number((ts as any).nanos ?? 0);
  const ms = seconds * 1000 + Math.floor(nanos / 1_000_000);
  return new Date(ms);
}

function timestampToLocale(ts?: Timestamp | null): string {
  const d = timestampToDate(ts);
  return d ? d.toLocaleString() : "";
}

interface LabelEntry {
  key: string;
  value: string;
}

export const TokenPage = () => {
  const tokenClient = useClient(TokenService)
  const configClient = useClient(ConfigService)
  const navigate = useNavigate()

  const [tokensState, setTokensState] = useState<BootstrapToken[]>([])
  const [configsState, setConfigsState] = useState<ConfigReference[]>([])
  const [deleteModalOpened, { open: openDeleteModal, close: closeDeleteModal }] = useDisclosure(false)
  const [createModalOpened, { open: openCreateModal, close: closeCreateModal }] = useDisclosure(false)
  const [tokenToDelete, setTokenToDelete] = useState<string | null>(null)

  // Create token form state
  const [selectedConfig, setSelectedConfig] = useState<string | null>(null)
  const [labels, setLabels] = useState<LabelEntry[]>([])
  const [newLabelKey, setNewLabelKey] = useState('')
  const [newLabelValue, setNewLabelValue] = useState('')

  const handleListConfigs = useCallback(async () => {
    try {
      const response = await configClient.listConfigs({})
      setConfigsState(response.configs)
    } catch (error) {
      notifyGRPCError("Failed to list configs", error)
    }
  }, [configClient])

  const handleListTokens = useCallback(async () => {
    try {
      const response = await tokenClient.listTokens({})
      setTokensState(response.tokens)
    } catch (error) {
      notifyGRPCError("Failed to list tokens", error)
    }
  }, [tokenClient])

  const handleAddLabel = useCallback(() => {
    if (newLabelKey.trim() && newLabelValue.trim()) {
      const newLabel = { key: newLabelKey.trim(), value: newLabelValue.trim() }
      setLabels(prev => {
        const updated = [...prev, newLabel]
        return updated
      })
      setNewLabelKey('')
      setNewLabelValue('')
    }

  }, [newLabelKey, newLabelValue])

  const handleRemoveLabel = useCallback((index: number) => {
    setLabels(prev => prev.filter((_, i) => i !== index))
  }, [])

  const resetCreateForm = useCallback(() => {
    setSelectedConfig(null)
    setLabels([])
    setNewLabelKey('')
    setNewLabelValue('')
  }, [])

  const handleOpenCreateModal = useCallback(() => {
    resetCreateForm()
    handleListConfigs()
    openCreateModal()
  }, [resetCreateForm, handleListConfigs, openCreateModal])

  const handleCreateToken = useCallback(async () => {
    try {
      const labelsMap: { [key: string]: string } = {}
      labels.forEach(({ key, value }) => {
        labelsMap[key] = value
      })
      const request = create(CreateTokenRequestSchema, {
        TTL: {
          seconds: BigInt(600),
        },
        configReference: selectedConfig || undefined,
        labels: labelsMap,
      })
      await tokenClient.createToken(request)
      notifications.show({
        title: "Token successfully created",
        message: 'Bootstrap token successfully created',
        icon: <CheckCircledIcon />,
      })
      closeCreateModal()
      handleListTokens()
    } catch (error) {
      notifyGRPCError("Create token error", error)
    }
  }, [tokenClient, handleListTokens, selectedConfig, labels, closeCreateModal])

  const handleDeleteToken = useCallback(async () => {
    if (!tokenToDelete) return
    try {
      await tokenClient.deleteToken({ ID: tokenToDelete })
      notifications.show({
        title: "Token deleted",
        message: "Token has been removed",
        icon: <CheckCircledIcon />,
      })
      handleListTokens()
    } catch (error) {
      notifyGRPCError("Failed to delete token", error)
    } finally {
      closeDeleteModal()
      setTokenToDelete(null)
    }
  }, [tokenClient, handleListTokens, tokenToDelete, closeDeleteModal])

  const confirmDelete = useCallback((tokenID: string) => {
    setTokenToDelete(tokenID)
    openDeleteModal()
  }, [openDeleteModal])

  const tokenColumns = useMemo<ColumnConfig<BootstrapToken>[]>(() => [
    { key: 'ID', label: 'ID', visible: true },
    { key: 'Secret', label: 'Token', visible: true },
    {
      key: 'configReference',
      label: 'Config',
      visible: true,
      render: (value: string | undefined) => (
        value ? (
          <Badge variant="light" color="blue">{value}</Badge>
        ) : (
          <Badge variant="light" color="gray">none</Badge>
        )
      )
    },
    {
      key: 'labels',
      label: 'Labels',
      visible: true,
      render: (value: { [key: string]: string } | undefined) => {
        if (!value || Object.keys(value).length === 0) {
          return <Text size="sm" c="dimmed">-</Text>
        }
        const entries = Object.entries(value)
        const displayCount = 2
        const displayedLabels = entries.slice(0, displayCount)
        const remainingCount = entries.length - displayCount
        return (
          <Group gap="xs" wrap="nowrap" justify="center">
            {displayedLabels.map(([k, v]) => (
              <Badge key={k} variant="outline" size="sm">
                {k}={v}
              </Badge>
            ))}
            {remainingCount > 0 && (
              <Badge variant="light" color="gray" size="sm">
                +{remainingCount} more
              </Badge>
            )}
          </Group>
        )
      }
    },
    {
      key: 'Expiry',
      label: 'Expires at',
      render: (value: Timestamp) => <div>{timestampToLocale(value)}</div>
    },
    {
      key: 'TTL',
      label: 'Actions',
      visible: true,
      render: (_value, row: BootstrapToken) => (
        <Group gap="xs">
          <ActionIcon
            color="blue"
            variant="subtle"
            onClick={() => navigate({ to: '/tokens/$tokenId', params: { tokenId: row.ID } })}
          >
            <EyeOpenIcon />
          </ActionIcon>
          <ActionIcon color="red" variant="subtle" onClick={() => confirmDelete(row.ID)}>
            <TrashIcon />
          </ActionIcon>
        </Group>
      )
    },
  ], [confirmDelete, navigate])

  useEffect(() => {
    handleListTokens()
  }, [handleListTokens])

  return (
    <Box>
      <Box style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
        <Button onClick={handleOpenCreateModal}>
          Create Token
        </Button>
      </Box>
      <Table<BootstrapToken> title="Tokens" data={tokensState} columns={tokenColumns} rowKey="ID" />

      <Modal opened={createModalOpened} onClose={closeCreateModal} title="Create Token" size="md">
        <Stack gap="md">
          <Select
            label="Associated Config"
            placeholder="Select a config (optional)"
            data={configsState.map(c => ({ value: c.id, label: c.id }))}
            value={selectedConfig}
            onChange={setSelectedConfig}
            clearable
          />

          <Box>
            <Text size="sm" fw={500} mb="xs">Labels</Text>
            {labels.map((label, index) => (
              <Group key={index} mb="xs">
                <TextInput
                  value={label.key}
                  readOnly
                  style={{ flex: 1 }}
                  size="sm"
                />
                <TextInput
                  value={label.value}
                  readOnly
                  style={{ flex: 1 }}
                  size="sm"
                />
                <ActionIcon color="red" variant="subtle" onClick={() => handleRemoveLabel(index)}>
                  <Cross2Icon />
                </ActionIcon>
              </Group>
            ))}
            <Group>
              <TextInput
                placeholder="Key"
                value={newLabelKey}
                onChange={(e) => setNewLabelKey(e.currentTarget.value)}
                style={{ flex: 1 }}
                size="sm"
              />
              <TextInput
                placeholder="Value"
                value={newLabelValue}
                onChange={(e) => setNewLabelValue(e.currentTarget.value)}
                style={{ flex: 1 }}
                size="sm"
              />
              <ActionIcon color="blue" variant="light" onClick={handleAddLabel}>
                <PlusIcon />
              </ActionIcon>
            </Group>
          </Box>

          <Group justify="flex-end" mt="md">
            <Button variant="default" onClick={closeCreateModal}>Cancel</Button>
            <Button onClick={handleCreateToken}>Create</Button>
          </Group>
        </Stack>
      </Modal>

      <Modal opened={deleteModalOpened} onClose={closeDeleteModal} title="Confirm Delete">
        <Text>Are you sure you want to delete this token? This action cannot be undone.</Text>
        <Group justify="flex-end" mt="md">
          <Button variant="default" onClick={closeDeleteModal}>Cancel</Button>
          <Button color="red" onClick={handleDeleteToken}>Delete</Button>
        </Group>
      </Modal>
    </Box>
  )
}
