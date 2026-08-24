import { Badge, Text, Code, Stack, Anchor } from '@mantine/core';
import { Link } from '@tanstack/react-router';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import type { Event, EventRef } from '../../gen/api/pkg/api/event/v1alpha1/event_pb';
import { EventSeverity } from '../../gen/api/pkg/api/event/v1alpha1/event_pb';
import { CollectorDescriptionSchema } from '../../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';
import { getEntityTypeByUrl } from '../../resources/entityTypes';
import type { ColumnConfig } from '../Table';

const severityDisplay: Record<EventSeverity, { label: string; color: string }> = {
    [EventSeverity.INFO]: { label: 'Info', color: 'blue' },
    [EventSeverity.WARN]: { label: 'Warn', color: 'yellow' },
    [EventSeverity.ERROR]: { label: 'Error', color: 'red' },
};

export const SeverityBadge = ({ severity }: { severity: EventSeverity }) => {
    const display = severityDisplay[severity] ?? { label: 'Unknown', color: 'gray' };
    return <Badge color={display.color} variant="light">{display.label}</Badge>;
};

const formatReportedAt = (event: Event) =>
    event.reportedAt ? timestampDate(event.reportedAt).toLocaleString() : '—';

const shortTypeUrl = (typeUrl: string) => typeUrl.split('/').pop() ?? typeUrl;

const COLLECTOR_TYPE_URL = `type.googleapis.com/${CollectorDescriptionSchema.typeName}`;

const ObjectRefLabel = ({ objectRef }: { objectRef: EventRef }) => (
    <>
        <Code>{shortTypeUrl(objectRef.typeUrl)}</Code> {objectRef.key}
    </>
);

const ObjectRefLink = ({ objectRef }: { objectRef: EventRef }) => {
    const label = <ObjectRefLabel objectRef={objectRef} />;

    if (objectRef.typeUrl === COLLECTOR_TYPE_URL) {
        return (
            <Anchor
                size="sm"
                renderRoot={(props) => (
                    <Link to="/deployments/$agentId" params={{ agentId: objectRef.key }} {...props} />
                )}
            >
                {label}
            </Anchor>
        );
    }

    const entityType = getEntityTypeByUrl(objectRef.typeUrl);
    if (entityType) {
        return (
            <Anchor
                size="sm"
                renderRoot={(props) => (
                    <Link
                        to="/resources/$type/editor"
                        params={{ type: entityType.slug }}
                        search={{ key: objectRef.key }}
                        {...props}
                    />
                )}
            >
                {label}
            </Anchor>
        );
    }

    return <Text size="sm">{label}</Text>;
};

const ObjectRefs = ({ refs }: { refs: EventRef[] }) => {
    if (refs.length === 0) return <Text size="sm" c="dimmed">—</Text>;
    return (
        <Stack gap={2}>
            {refs.map((ref) => (
                <ObjectRefLink key={`${ref.typeUrl}/${ref.key}`} objectRef={ref} />
            ))}
        </Stack>
    );
};

export function buildEventColumns(): ColumnConfig<Event>[] {
    return [
        {
            key: 'reportedAt',
            label: 'Reported',
            render: (_: unknown, row: Event) => <Text size="sm">{formatReportedAt(row)}</Text>,
        },
        {
            key: 'severity',
            label: 'Severity',
            render: (_: unknown, row: Event) => <SeverityBadge severity={row.severity} />,
        },
        {
            key: 'group',
            label: 'Group',
            render: (_: unknown, row: Event) => <Text size="sm">{row.group || '—'}</Text>,
        },
        {
            key: 'objectRefs',
            label: 'Objects',
            render: (_: unknown, row: Event) => <ObjectRefs refs={row.objectRefs} />,
        },
        {
            key: 'reason',
            label: 'Reason',
            render: (_: unknown, row: Event) => <Text size="sm">{row.details?.reason || '—'}</Text>,
        },
    ];
}
