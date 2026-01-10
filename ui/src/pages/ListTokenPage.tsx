import {
    Table,
    type ColumnConfig
} from '../components/Table'
import { Box } from '@mui/material';
import type {   Timestamp } from "@bufbuild/protobuf/wkt";
import {Button} from '@mantine/core';
import { notifications } from '@mantine/notifications';
import { ConnectError } from "@connectrpc/connect";
import { Code } from "@connectrpc/connect";


import { useClient } from '../api';
import { TokenService } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb';
import {useState, useEffect} from 'react';
import type { BootstrapToken } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb'
import { CheckCircledIcon, CrossCircledIcon } from '@radix-ui/react-icons';
import { notifyGRPCError } from '../api/notifications';

const tokenColumns: ColumnConfig<BootstrapToken>[] = [
      { key: 'ID', label: 'ID', visible: true },
      { key: 'Secret', label: 'token', visible: true },
      { key : 'Expiry', 
        label : "Expires at", 
        render: (value: Timestamp, row: BootstrapToken) => {
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

function timestampToStringISO(ts?: Timestamp | null): string {
  const d = timestampToDate(ts);
  return d ? d.toISOString() : "";
}

function timestampToLocale(ts?: Timestamp | null): string {
  const d = timestampToDate(ts);
  return d ? d.toLocaleString() : "";
}

export const TokenPage = () => {
  const client = useClient(TokenService)

  const [configState, setState] = useState<BootstrapToken[]>([])

  const handleListConfigs = async () => {
    try {
      const response = await client.listTokens({})
      setState(response.tokens)
    } catch (error) {
      notifyGRPCError("Failed to list tokens", error)
    }
  }
   const handleCreateToken = async () => {
    try {
      await client.createToken({
        TTL: {
            seconds: BigInt(600),
        },
      })
      notifications.show({
        title: "Token successfully created",
        message: 'Bootstrap token successfully created',
        icon: <CheckCircledIcon/>,
      })
      
      // Refresh the list after creating
      handleListConfigs()
    } catch (error) {
      notifyGRPCError("Create token error", error)
    }
  }

    useEffect(() => {
        handleListConfigs()
    }, [])
     return (
        <Box>
            <Box sx={{ mb: 2, display: 'flex', justifyContent: 'flex-end' }}>
                <Button 
                    variant="contained" 
                    onClick={handleCreateToken}
                >
                    Create Token
                </Button>
            </Box>
            <Table<BootstrapToken> title="Tokens" data={configState} columns={tokenColumns} />
        </Box>
    )
}