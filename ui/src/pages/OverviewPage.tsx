import { useCallback, useEffect, useMemo, useState } from 'react';
import { Group, Paper, Select, SimpleGrid, Stack, Text, Title } from '@mantine/core';
import { CollectorService, CollectorState } from '../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';
import { EventService, EventSeverity } from '../gen/api/pkg/api/event/v1alpha1/event_pb';
import type { Event } from '../gen/api/pkg/api/event/v1alpha1/event_pb';
import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';
import { Table } from '../components/Table';
import { buildEventColumns } from '../components/events/eventColumns';
import { useEventWatch } from '../hooks/useEventWatch';

const EVENT_LIMITS = [25, 50, 100, 250];
const ALL_GROUPS = 'all';
const ALL_SEVERITIES = 'all';

const noSelectStyles = { input: { userSelect: 'none' as const }, option: { userSelect: 'none' as const } };

const SEVERITY_OPTIONS = [
    { value: ALL_SEVERITIES, label: 'All' },
    { value: String(EventSeverity.INFO), label: 'Info' },
    { value: String(EventSeverity.WARN), label: 'Warn' },
    { value: String(EventSeverity.ERROR), label: 'Error' },
];

export type StatCardProps = {
    label: string;
    value: number | string;
    hint?: string;
};

export const StatCard = ({ label, value, hint }: StatCardProps) => (
    <Paper p="md" withBorder>
        <Stack gap={4}>
            <Text size="sm" c="dimmed">{label}</Text>
            <Text fz={32} fw={700} lh={1}>{value}</Text>
            {hint ? <Text size="xs" c="dimmed">{hint}</Text> : null}
        </Stack>
    </Paper>
);

export const OverviewPage = () => {
    const collectorClient = useClient(CollectorService);
    const eventClient = useClient(EventService);

    const [collectorCount, setCollectorCount] = useState(0);
    const [connectedCount, setConnectedCount] = useState(0);
    const [groups, setGroups] = useState<string[]>([]);

    const [severity, setSeverity] = useState(ALL_SEVERITIES);
    const [group, setGroup] = useState(ALL_GROUPS);
    const [limit, setLimit] = useState(EVENT_LIMITS[0]);

    const loadCollectors = useCallback(async () => {
        try {
            // FIXME: slow
            const response = await collectorClient.listCollectors({ withStatus: true });
            setCollectorCount(response.collectors.length);
            setConnectedCount(
                response.collectors.filter((c) => c.status?.connStatus?.state === CollectorState.CONNECTED).length,
            );
        } catch (error) {
            notifyGRPCError('Failed to list collectors', error);
        }
    }, [collectorClient]);

    const loadGroups = useCallback(async () => {
        try {
            const response = await eventClient.getGroups({});
            setGroups(response.groups);
        } catch (error) {
            notifyGRPCError('Failed to list event groups', error);
        }
    }, [eventClient]);

    useEffect(() => {
        loadCollectors();
        loadGroups();
    }, [loadCollectors, loadGroups]);

    const events = useEventWatch({
        severity: severity === ALL_SEVERITIES ? undefined : (Number(severity) as EventSeverity),
        group: group === ALL_GROUPS ? '' : group,
        limit,
    });

    const eventColumns = useMemo(() => buildEventColumns(), []);
    const groupOptions = useMemo(
        () => [{ value: ALL_GROUPS, label: 'All' }, ...groups.map((g) => ({ value: g, label: g }))],
        [groups],
    );
    const warnCount = events.filter((e) => e.severity !== EventSeverity.INFO).length;

    return (
        <Stack gap="lg" style={{ flex: 1, minHeight: 0 }}>
            <Group justify="space-between" align="baseline">
                <Title order={2}>Overview</Title>
            </Group>

            <SimpleGrid cols={{ base: 1, sm: 3 }}>
                <StatCard label="Collectors configured" value={collectorCount} />
                <StatCard
                    label="Collectors connected"
                    value={connectedCount}
                    hint={`${collectorCount - connectedCount} not connected`}
                />
                <StatCard
                    label="Recent events"
                    value={events.length}
                    hint={`${warnCount} at warn or above`}
                />
            </SimpleGrid>

            <Group gap="sm" justify="center">
                <Select
                    label="Severity"
                    data={SEVERITY_OPTIONS}
                    value={severity}
                    onChange={(value) => setSeverity(value ?? ALL_SEVERITIES)}
                    allowDeselect={false}
                    styles={noSelectStyles}
                    w={180}
                />
                <Select
                    label="Group"
                    data={groupOptions}
                    value={group}
                    onChange={(value) => setGroup(value ?? ALL_GROUPS)}
                    allowDeselect={false}
                    styles={noSelectStyles}
                    w={180}
                />
                <Select
                    label="Limit"
                    data={EVENT_LIMITS.map((n) => String(n))}
                    value={String(limit)}
                    onChange={(value) => setLimit(Number(value))}
                    allowDeselect={false}
                    styles={noSelectStyles}
                    w={110}
                />
            </Group>

            <Table<Event>
                title="Recent events"
                data={events}
                columns={eventColumns}
                rowKey={(_, index) => index}
            />
        </Stack>
    );
};
