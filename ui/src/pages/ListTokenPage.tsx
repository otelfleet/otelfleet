import {
    Table,
    type ColumnConfig
} from '../components/Table'
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { Box, Button, ActionIcon, Modal, Group, Text } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { useClient } from '../api';
import { TokenService } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb';
import { useState, useEffect, useCallback, useMemo } from 'react';
import type { BootstrapToken } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb'
import { CheckCircledIcon, TrashIcon } from '@radix-ui/react-icons';
import { notifyGRPCError } from '../api/notifications';

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

export const TokenPage = () => {
  const client = useClient(TokenService)

  const [tokensState, setTokensState] = useState<BootstrapToken[]>([])
  const [deleteModalOpened, { open: openDeleteModal, close: closeDeleteModal }] = useDisclosure(false)
  const [tokenToDelete, setTokenToDelete] = useState<string | null>(null)

  const handleListTokens = useCallback(async () => {
    try {
      const response = await client.listTokens({})
      setTokensState(response.tokens)
    } catch (error) {
      notifyGRPCError("Failed to list tokens", error)
    }
  }, [client])

  const handleCreateToken = useCallback(async () => {
    try {
      await client.createToken({
        TTL: {
          seconds: BigInt(600),
        },
      })
      notifications.show({
        title: "Token successfully created",
        message: 'Bootstrap token successfully created',
        icon: <CheckCircledIcon />,
      })
      handleListTokens()
    } catch (error) {
      notifyGRPCError("Create token error", error)
    }
  }, [client, handleListTokens])

  const handleDeleteToken = useCallback(async () => {
    if (!tokenToDelete) return
    try {
      await client.deleteToken({ ID: tokenToDelete })
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
  }, [client, handleListTokens, tokenToDelete, closeDeleteModal])

  const confirmDelete = useCallback((tokenID: string) => {
    setTokenToDelete(tokenID)
    openDeleteModal()
  }, [openDeleteModal])

  const tokenColumns = useMemo<ColumnConfig<BootstrapToken>[]>(() => [
    { key: 'ID', label: 'ID', visible: true },
    { key: 'Secret', label: 'Token', visible: true },
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
        <ActionIcon color="red" variant="subtle" onClick={() => confirmDelete(row.ID)}>
          <TrashIcon />
        </ActionIcon>
      )
    },
  ], [confirmDelete])

  useEffect(() => {
    handleListTokens()
  }, [handleListTokens])

  return (
    <Box>
      <Box style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end' }}>
        <Button onClick={handleCreateToken}>
          Create Token
        </Button>
      </Box>
      <Table<BootstrapToken> title="Tokens" data={tokensState} columns={tokenColumns} rowKey="ID" />

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
