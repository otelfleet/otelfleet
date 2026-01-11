import {
    Table,
    type ColumnConfig
} from '../components/Table'
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { Box, Button } from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { useClient } from '../api';
import { TokenService } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb';
import { useState, useEffect, useCallback } from 'react';
import type { BootstrapToken } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb'
import { CheckCircledIcon } from '@radix-ui/react-icons';
import { notifyGRPCError } from '../api/notifications';

const tokenColumns: ColumnConfig<BootstrapToken>[] = [
      { key: 'ID', label: 'ID', visible: true },
      { key: 'Secret', label: 'token', visible: true },
      { key : 'Expiry',
        label : "Expires at",
        render: (value: Timestamp) => {
          return <div>
            {timestampToLocale(value)}
          </div>
        }
      },
]

function timestampToDate(ts?: Timestamp | null): Date | null {
  if (!ts) return null;

  // seconds can be number, string, bigint, or a Long-like object
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

  // convert to ms, rounding nanos -> ms
  const ms = seconds * 1000 + Math.floor(nanos / 1_000_000);
  return new Date(ms);
}

function timestampToLocale(ts?: Timestamp | null): string {
  const d = timestampToDate(ts);
  return d ? d.toLocaleString() : "";
}

export const TokenPage = () => {
  const client = useClient(TokenService)

  const [configState, setState] = useState<BootstrapToken[]>([])

  const handleListTokens = useCallback(async () => {
    try {
      const response = await client.listTokens({})
      setState(response.tokens)
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
      <Table<BootstrapToken> title="Tokens" data={configState} columns={tokenColumns} rowKey="ID" />
    </Box>
  )
}