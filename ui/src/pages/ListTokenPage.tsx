import {
  Table,
  type ColumnConfig
} from '../components/Table'
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { Box, Button, ActionIcon, Modal, Group, Text, Badge } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { useClient } from '../api';
import { TokenService } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb';
import { useState, useEffect, useCallback, useMemo } from 'react';
import type { BootstrapToken } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb';
import { CheckCircledIcon, TrashIcon, EyeOpenIcon, CopyIcon } from '@radix-ui/react-icons';
import { notifyGRPCError } from '../api/notifications';
import { Link, useNavigate } from '@tanstack/react-router';

function timestampToDate(ts?: Timestamp | null): Date | null {
  if (!ts) return null;

  const rawSec = (ts as { seconds?: number | string | bigint | { toNumber(): number } }).seconds ?? 0;
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

  const nanos = Number((ts as { nanos?: number }).nanos ?? 0);
  const ms = seconds * 1000 + Math.floor(nanos / 1_000_000);
  return new Date(ms);
}

function timestampToLocale(ts?: Timestamp | null): string {
  const d = timestampToDate(ts);
  return d ? d.toLocaleString() : "";
}

export const TokenPage = () => {
  const tokenClient = useClient(TokenService)
  const navigate = useNavigate()

  const [tokensState, setTokensState] = useState<BootstrapToken[]>([])
  const [deleteModalOpened, { open: openDeleteModal, close: closeDeleteModal }] = useDisclosure(false)
  const [tokenToDelete, setTokenToDelete] = useState<string | null>(null)

  const handleListTokens = useCallback(async () => {
    try {
      const response = await tokenClient.listTokens({})
      setTokensState(response.tokens)
    } catch (error) {
      notifyGRPCError("Failed to list tokens", error)
    }
  }, [tokenClient])

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

  const handleCopyToken = useCallback((token: string) => {
    navigator.clipboard.writeText(token).then(() => {
      notifications.show({
        title: "Copied",
        message: "Bootstrap token copied to clipboard",
        icon: <CheckCircledIcon />,
      })
    }).catch(() => {
      notifications.show({
        title: "Copy failed",
        message: "Failed to copy token to clipboard",
        color: "red",
      })
    })
  }, [])

  const tokenColumns = useMemo<ColumnConfig<BootstrapToken>[]>(() => [
    { key: 'ID', label: 'ID', visible: false },
    {
      key: 'Secret',
      label: 'Bootstrap Token',
      visible: true,
      render: (_value: string, row: BootstrapToken) => {
        const fullToken = `${row.ID}.${row.Secret}`
        return (
          <Group gap="xs" wrap="nowrap" justify="center">
            <Text size="sm" style={{ fontFamily: 'monospace' }}>{fullToken}</Text>
            <ActionIcon
              color="gray"
              variant="subtle"
              size="sm"
              onClick={() => handleCopyToken(fullToken)}
              title="Copy to clipboard"
            >
              <CopyIcon />
            </ActionIcon>
          </Group>
        )
      }
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
  ], [confirmDelete, navigate, handleCopyToken])

  useEffect(() => {
    handleListTokens()
  }, [handleListTokens])

  return (
    <Box style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      <Box style={{ marginBottom: 16, display: 'flex', justifyContent: 'flex-end', flexShrink: 0 }}>
        <Link to="/tokens/create" style={{ display: 'inline-block' }}>
          <Button>Create Token</Button>
        </Link>
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
