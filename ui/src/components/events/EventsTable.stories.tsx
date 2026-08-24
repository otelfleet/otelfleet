import type { Meta, StoryObj } from '@storybook/tanstack-react';
import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import {
    EventSchema,
    EventDetailsSchema,
    EventRefSchema,
    EventSeverity,
} from '../../gen/api/pkg/api/event/v1alpha1/event_pb';
import type { Event } from '../../gen/api/pkg/api/event/v1alpha1/event_pb';
import { CollectorDescriptionSchema } from '../../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';
import { CollectorConfigSchema } from '../../gen/api/pkg/api/resources/v1alpha1/resources_pb';
import { Table } from '../Table';
import { buildEventColumns } from './eventColumns';

const COLLECTOR_TYPE_URL = `type.googleapis.com/${CollectorDescriptionSchema.typeName}`;
const COLLECTOR_CONFIG_TYPE_URL = `type.googleapis.com/${CollectorConfigSchema.typeName}`;

const mockEvent = (
    severity: EventSeverity,
    group: string,
    reason: string,
    refs: { typeUrl: string; key: string }[],
    minutesAgo: number,
): Event =>
    create(EventSchema, {
        severity,
        group,
        reportedAt: timestampFromDate(new Date(Date.UTC(2026, 0, 1, 12, 0) - minutesAgo * 60_000)),
        details: create(EventDetailsSchema, { reason }),
        objectRefs: refs.map((ref) => create(EventRefSchema, ref)),
    });

const EVENTS: Event[] = [
    mockEvent(EventSeverity.INFO, 'collector', 'Collector connected', [{ typeUrl: COLLECTOR_TYPE_URL, key: 'collector-a' }], 1),
    mockEvent(EventSeverity.WARN, 'config', 'Config assignment ambiguous', [
        { typeUrl: COLLECTOR_TYPE_URL, key: 'collector-a' },
        { typeUrl: COLLECTOR_CONFIG_TYPE_URL, key: 'edge-gateway' },
    ], 12),
    mockEvent(EventSeverity.ERROR, 'collector', 'Failed to apply remote config', [{ typeUrl: 'type.googleapis.com/unknown.v1.Thing', key: 'thing-1' }], 45),
];

const meta = {
    title: 'Events/EventsTable',
    render: (args) => <Table<Event> {...args} />,
    args: {
        title: 'Recent events',
        data: EVENTS,
        columns: buildEventColumns(),
        rowKey: (_: Event, index: number) => index,
    },
} satisfies Meta<React.ComponentProps<typeof Table<Event>>>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Empty: Story = {
    args: { data: [] },
};
