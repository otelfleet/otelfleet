import { useEffect, useState, useCallback } from 'react';
import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { TokenService } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb';
import type { BootstrapToken } from '../gen/api/pkg/api/bootstrap/v1alpha1/bootstrap_pb';
import type { Timestamp, Duration } from '@bufbuild/protobuf/wkt';
import {
    Paper,
    Title,
    Text,
    Badge,
    Group,
    Stack,
    Table,
    Loader,
    Center,
    Alert,
} from '@mantine/core';
import { AlertCircle } from 'react-feather';

interface TokenDetailPageProps {
    tokenId: string;
}

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
    return d ? d.toLocaleString() : "N/A";
}

function durationToString(duration?: Duration | null): string {
    if (!duration) return "N/A";
    const seconds = Number(duration.seconds ?? 0);
    if (seconds < 60) return `${seconds} seconds`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)} minutes`;
    return `${Math.floor(seconds / 3600)} hours`;
}

export function TokenDetailPage({ tokenId }: TokenDetailPageProps) {
    const tokenClient = useClient(TokenService);
    const [token, setToken] = useState<BootstrapToken | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const fetchTokenData = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            // Since there's no getToken API, we fetch all tokens and find the one we need
            const response = await tokenClient.listTokens({});
            const foundToken = response.tokens.find(t => t.ID === tokenId);
            if (!foundToken) {
                setError('Token not found');
                return;
            }
            setToken(foundToken);
        } catch (err) {
            notifyGRPCError('Failed to fetch token details', err);
            setError('Failed to load token details');
        } finally {
            setLoading(false);
        }
    }, [tokenId, tokenClient]);

    useEffect(() => {
        fetchTokenData();
    }, [fetchTokenData]);

    if (loading) {
        return (
            <Center style={{ height: '100%', minHeight: 400 }}>
                <Loader size="lg" />
            </Center>
        );
    }

    if (error) {
        return (
            <Alert color="red" title="Error" icon={<AlertCircle size={16} />}>
                {error}
            </Alert>
        );
    }

    return (
        <Stack gap="md" style={{ height: '100%' }}>
            <TokenHeader token={token} />
            <DetailsTab token={token} />
        </Stack>
    );
}

function TokenHeader({ token }: { token: BootstrapToken | null }) {
    const isExpired = token?.Expiry ? timestampToDate(token.Expiry)! < new Date() : false;

    return (
        <Paper p="md" withBorder>
            <Group justify="space-between" align="flex-start">
                <Stack gap="xs">
                    <Title order={2}>Bootstrap Token</Title>
                    <Text size="sm" c="dimmed">ID: {token?.ID || 'N/A'}</Text>
                </Stack>
                <Group gap="sm">
                    <Badge
                        color={isExpired ? 'red' : 'green'}
                        variant="filled"
                        size="lg"
                    >
                        {isExpired ? 'Expired' : 'Active'}
                    </Badge>
                </Group>
            </Group>
        </Paper>
    );
}

function DetailsTab({ token }: { token: BootstrapToken | null }) {
    if (!token) {
        return (
            <Alert color="gray" title="No Token Data">
                No token information available.
            </Alert>
        );
    }

    const labels = token.labels || {};
    const labelEntries = Object.entries(labels);

    return (
        <Stack gap="md">
            <Paper p="md" withBorder>
                <Title order={4} mb="md">Token Information</Title>
                <Table>
                    <Table.Tbody>
                        <Table.Tr>
                            <Table.Td width={200}>
                                <Text fw={500}>Token ID</Text>
                            </Table.Td>
                            <Table.Td>
                                <Text style={{ fontFamily: 'monospace' }}>{token.ID}</Text>
                            </Table.Td>
                        </Table.Tr>
                        <Table.Tr>
                            <Table.Td>
                                <Text fw={500}>Secret</Text>
                            </Table.Td>
                            <Table.Td>
                                <Text style={{ fontFamily: 'monospace' }}>{token.Secret}</Text>
                            </Table.Td>
                        </Table.Tr>
                        <Table.Tr>
                            <Table.Td>
                                <Text fw={500}>TTL</Text>
                            </Table.Td>
                            <Table.Td>
                                <Text>{durationToString(token.TTL)}</Text>
                            </Table.Td>
                        </Table.Tr>
                        <Table.Tr>
                            <Table.Td>
                                <Text fw={500}>Expires At</Text>
                            </Table.Td>
                            <Table.Td>
                                <Text>{timestampToLocale(token.Expiry)}</Text>
                            </Table.Td>
                        </Table.Tr>
                    </Table.Tbody>
                </Table>
            </Paper>

            <Paper p="md" withBorder>
                <Title order={4} mb="xs">Labels</Title>
                <Text size="sm" c="dimmed" mb="md">Key-value pairs associated with this token</Text>
                {labelEntries.length === 0 ? (
                    <Text size="sm" c="dimmed">No labels defined</Text>
                ) : (
                    <Table striped highlightOnHover>
                        <Table.Thead>
                            <Table.Tr>
                                <Table.Th>Key</Table.Th>
                                <Table.Th>Value</Table.Th>
                            </Table.Tr>
                        </Table.Thead>
                        <Table.Tbody>
                            {labelEntries.map(([key, value]) => (
                                <Table.Tr key={key}>
                                    <Table.Td>
                                        <Text size="sm" fw={500}>{key}</Text>
                                    </Table.Td>
                                    <Table.Td>
                                        <Text size="sm" style={{ fontFamily: 'monospace' }}>{value}</Text>
                                    </Table.Td>
                                </Table.Tr>
                            ))}
                        </Table.Tbody>
                    </Table>
                )}
            </Paper>
        </Stack>
    );
}
