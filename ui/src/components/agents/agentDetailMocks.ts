// Shared mock data for the agent-detail Storybook stories. Built with the real
// protobuf schemas via `create(...)` so the shapes match production messages.
import { create } from '@bufbuild/protobuf';
import {
    CollectorDescriptionSchema,
    CollectorStatusSchema,
    CollectorState,
    ConfigSyncStatus,
    EffectiveConfigSchema,
} from '../../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';
import type {
    CollectorDescription,
    CollectorStatus,
    ComponentHealth,
    EffectiveConfig,
} from '../../gen/api/pkg/api/deployment/v1alpha1/deployment_pb';

// Fixed timestamps so stories render deterministically (no wall-clock).
const START_TIME_NANOS = 1_704_067_200_000_000_000n;
const STATUS_TIME_NANOS = 1_704_070_800_000_000_000n; // +1h

/** A string-valued KeyValue init, as accepted by `create(CollectorDescriptionSchema, ...)`. */
const strAttr = (key: string, value: string) => ({
    key,
    value: { value: { case: 'stringValue' as const, value } },
});

export function mockAgent(overrides?: {
    id?: string;
    friendlyName?: string;
    identifying?: Array<[string, string]>;
    nonIdentifying?: Array<[string, string]>;
    capabilities?: string[];
}): CollectorDescription {
    return create(CollectorDescriptionSchema, {
        id: overrides?.id ?? 'agent-web-01',
        friendlyName: overrides?.friendlyName ?? 'web-collector-01',
        identifyingAttributes: (overrides?.identifying ?? [
            ['service.name', 'otel-collector'],
            ['service.instance.id', '7f3c9a20-1e4b-4c8a-9f2d-2a1b3c4d5e6f'],
            ['service.version', '0.104.0'],
        ]).map(([k, v]) => strAttr(k, v)),
        nonIdentifyingAttributes: (overrides?.nonIdentifying ?? [
            ['os.type', 'linux'],
            ['host.arch', 'amd64'],
            ['host.name', 'ip-10-0-1-42'],
        ]).map(([k, v]) => strAttr(k, v)),
        capabilities: overrides?.capabilities ?? [
            'AcceptsRemoteConfig',
            'ReportsEffectiveConfig',
            'ReportsHealth',
            'ReportsOwnMetrics',
        ],
    });
}

export function mockStatus(overrides?: {
    state?: CollectorState;
    healthy?: boolean;
    hasHealth?: boolean;
    lastError?: string;
    statusMessage?: string;
    configSyncStatus?: ConfigSyncStatus;
    componentHealthMap?: { [key: string]: ComponentHealth };
    effectiveConfigYaml?: string;
}): CollectorStatus {
    const hasHealth = overrides?.hasHealth ?? true;
    return create(CollectorStatusSchema, {
        connStatus: {
            state: overrides?.state ?? CollectorState.CONNECTED,
        },
        syncStatus: {
            status: overrides?.configSyncStatus ?? ConfigSyncStatus.IN_SYNC,
        },
        health: hasHealth
            ? {
                healthy: overrides?.healthy ?? true,
                status: overrides?.statusMessage ?? 'Everything is running',
                lastError: overrides?.lastError ?? '',
                startTimeUnixNano: START_TIME_NANOS,
                statusTimeUnixNano: STATUS_TIME_NANOS,
                componentHealthMap: overrides?.componentHealthMap ?? {},
            }
            : undefined,
        effectiveConfig: overrides?.effectiveConfigYaml
            ? {
                configMap: {
                    configMap: {
                        'collector.yaml': {
                            body: new TextEncoder().encode(overrides.effectiveConfigYaml),
                            contentType: 'text/yaml',
                        },
                    },
                },
            }
            : undefined,
    });
}

/** A nested component-health tree exercising the recursive component rows. */
export const NESTED_COMPONENT_HEALTH: { [key: string]: ComponentHealth } = create(
    CollectorStatusSchema,
    {
        health: {
            componentHealthMap: {
                'receiver:otlp': { healthy: true, status: 'Running' },
                'exporter:otlphttp': {
                    healthy: false,
                    status: 'Retrying',
                    lastError: 'connection refused to https://otel.example.com:4318',
                },
                'pipeline:traces': {
                    healthy: true,
                    status: 'Running',
                    componentHealthMap: {
                        'processor:batch': { healthy: true, status: 'Running' },
                        'processor:memory_limiter': { healthy: true, status: 'Running' },
                    },
                },
            },
        },
    },
).health!.componentHealthMap;

export const SAMPLE_EFFECTIVE_CONFIG = `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
processors:
  batch: {}
exporters:
  otlphttp:
    endpoint: https://otel-collector.example.com
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp]
`;

export function mockEffectiveConfig(files: { [name: string]: string }): EffectiveConfig {
    return create(EffectiveConfigSchema, {
        configMap: {
            configMap: Object.fromEntries(
                Object.entries(files).map(([name, body]) => [
                    name,
                    { body: new TextEncoder().encode(body), contentType: 'text/yaml' },
                ]),
            ),
        },
    });
}

/** Config history, newest-first as the API returns it, each revision swapping the exporter endpoint. */
export function mockHistory(count = 4): EffectiveConfig[] {
    return Array.from({ length: count }, (_, position) => count - 1 - position).map((revision) =>
        mockEffectiveConfig({
            'collector.yaml': SAMPLE_EFFECTIVE_CONFIG.replace(
                'https://otel-collector.example.com',
                `https://otel-collector-v${revision}.example.com`,
            ),
        }),
    );
}
