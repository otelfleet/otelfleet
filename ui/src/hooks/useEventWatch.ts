import { useEffect, useState } from 'react';
import { ConnectError, Code } from '@connectrpc/connect';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { EventService, EventSeverity } from '../gen/api/pkg/api/event/v1alpha1/event_pb';
import type { Event } from '../gen/api/pkg/api/event/v1alpha1/event_pb';
import { useClient } from '../api';
import { notifyGRPCError } from '../api/notifications';

export type EventWatchOptions = {
    severity?: EventSeverity;
    group: string;
    limit: number;
};

const reportedAtMillis = (event: Event) =>
    event.reportedAt ? timestampDate(event.reportedAt).getTime() : 0;

const mostRecent = (events: Event[], limit: number) =>
    events.slice().sort((a, b) => reportedAtMillis(b) - reportedAtMillis(a)).slice(0, limit);

export function useEventWatch({ severity, group, limit }: EventWatchOptions): Event[] {
    const client = useClient(EventService);
    const [events, setEvents] = useState<Event[]>([]);

    useEffect(() => {
        const abort = new AbortController();
        setEvents([]);

        const watch = async () => {
            try {
                for await (const response of client.watch({ severity, group }, { signal: abort.signal })) {
                    setEvents((current) => mostRecent([...response.events, ...current], limit));
                }
            } catch (error) {
                if (abort.signal.aborted || ConnectError.from(error).code === Code.Canceled) return;
                notifyGRPCError('Failed to watch events', error);
            }
        };
        watch();

        return () => abort.abort();
    }, [client, severity, group, limit]);

    return events;
}
