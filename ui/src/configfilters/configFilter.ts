import { create } from '@bufbuild/protobuf';
import { anyPack, anyUnpack, type Any } from '@bufbuild/protobuf/wkt';

import {
  ConfigFilterSchema,
  LabelFilterSchema,
  MatchType,
  type ConfigFilter,
} from '../gen/api/pkg/api/resources/v1alpha1/resources_pb';

export const CONFIG_FILTER_TYPE_URL = `type.googleapis.com/${ConfigFilterSchema.typeName}`;

export type LabelScope = 'identifying' | 'nonIdentifying' | 'otelfleet';

export interface LabelRow {
  type: MatchType;
  key: string;
  value: string;
}

export interface LabelFilterValues {
  scope: LabelScope;
  labels: LabelRow[];
}

export interface ConfigFilterValues {
  name: string;
  isDefault: boolean;
  requiresApproval: boolean;
  configRef: string;
  filters: LabelFilterValues[];
}

export const MATCH_TYPE_OPTIONS = [
  { value: String(MatchType.EQ), label: '==' },
  { value: String(MatchType.NEQ), label: '!=' },
  { value: String(MatchType.RE), label: '~=' },
  { value: String(MatchType.NR), label: '!~' },
];

export const LABEL_SCOPE_OPTIONS: { value: LabelScope; label: string }[] = [
  { value: 'identifying', label: 'Collector identity' },
  { value: 'nonIdentifying', label: 'Collector environment' },
  { value: 'otelfleet', label: 'User-defined' },
];

export const LABEL_SCOPE_DESCRIPTIONS: Record<LabelScope, string> = {
  identifying: 'Reported by the collector, the OpenTelemetryCollector information',
  nonIdentifying: 'Reported by the collector: the OpenTelemetryCollector host information',
  otelfleet: 'Labels you assign in OtelFleet',
};

// Defaults set by opampextension's createAgentDescription; agents may report more.
export const LABEL_KEY_SUGGESTIONS: Record<LabelScope, { key: string; description: string }[]> = {
  identifying: [
    { key: 'service.instance.id', description: 'Unique ID of the collector instance' },
    { key: 'service.name', description: 'Collector distribution' },
    { key: 'service.version', description: 'Version of the collector build' },
  ],
  nonIdentifying: [
    { key: 'host.arch', description: 'CPU architecture of the host, e.g. amd64' },
    { key: 'host.name', description: 'Hostname of the machine running the collector' },
    { key: 'os.description', description: 'Human-readable OS version, e.g. Alpine 3.24.1' },
    { key: 'os.type', description: 'Operating system, e.g. linux' },
  ],
  otelfleet: [],
};

export const LABEL_VALUE_SUGGESTIONS: Record<string, string[]> = {
  'service.name': ['otelcol', 'otelcol-contrib', 'otelcol-otlp', 'otelcol-k8s', 'otelcol-ebpf-profiler'],
// GOOS / GOARCH as reported by runtime.GOOS and runtime.GOARCH.
  'os.type': [
    'aix', 'android', 'darwin', 'dragonfly', 'freebsd', 'illumos', 'ios', 'js',
    'linux', 'netbsd', 'openbsd', 'plan9', 'solaris', 'wasip1', 'windows',
  ],
  'host.arch': [
    '386', 'amd64', 'arm', 'arm64', 'loong64', 'mips', 'mips64', 'mips64le',
    'mipsle', 'ppc64', 'ppc64le', 'riscv64', 's390x', 'wasm',
  ],
};

export function emptyConfigFilterValues(): ConfigFilterValues {
  return {
    name: '',
    isDefault: false,
    requiresApproval: false,
    configRef: '',
    filters: [],
  };
}

export function emptyLabelFilterValues(): LabelFilterValues {
  return { scope: 'identifying', labels: [emptyLabelRow()] };
}

export function emptyLabelRow(): LabelRow {
  return { type: MatchType.EQ, key: '', value: '' };
}

export function packConfigFilter(values: ConfigFilterValues): Any {
  const filters = values.filters.flatMap((filter) =>
    matchTypesOf(filter.labels).map((type) =>
      create(LabelFilterSchema, {
        type,
        ...labelsForScope(filter.scope, labelsOfType(filter.labels, type)),
      }),
    ),
  );

  return anyPack(
    ConfigFilterSchema,
    create(ConfigFilterSchema, {
      default: values.isDefault,
      approval: { requiresApproval: values.requiresApproval },
      collectorConfig: { configRef: values.configRef },
      filters,
    }),
  );
}

export function unpackConfigFilter(obj?: Any): ConfigFilter | undefined {
  if (!obj) return undefined;
  return anyUnpack(obj, ConfigFilterSchema);
}

export function toConfigFilterValues(name: string, filter?: ConfigFilter): ConfigFilterValues {
  if (!filter) return { ...emptyConfigFilterValues(), name };

  const filters = filter.filters.flatMap((labelFilter) =>
    (
      [
        ['identifying', labelFilter.opampIdLabels],
        ['nonIdentifying', labelFilter.opampNonIdLabels],
        ['otelfleet', labelFilter.otelfleetLabels],
      ] as const
    )
      .filter(([, labels]) => Object.keys(labels).length > 0)
      .map(([scope, labels]) => ({ scope, labels: labelRows(labels, labelFilter.type) })),
  );

  return {
    name,
    isDefault: filter.default ?? false,
    requiresApproval: filter.approval?.requiresApproval ?? false,
    configRef: filter.collectorConfig?.configRef ?? '',
    filters,
  };
}

export function countLabels(filter?: ConfigFilter): number {
  if (!filter) return 0;
  return filter.filters.reduce(
    (total, labelFilter) =>
      total +
      Object.keys(labelFilter.opampIdLabels).length +
      Object.keys(labelFilter.opampNonIdLabels).length +
      Object.keys(labelFilter.otelfleetLabels).length,
    0,
  );
}

function matchTypesOf(rows: LabelRow[]): MatchType[] {
  return [...new Set(rows.filter((row) => row.key !== '').map((row) => row.type))];
}

function labelsOfType(rows: LabelRow[], type: MatchType): { [key: string]: string } {
  const labels: { [key: string]: string } = {};
  for (const row of rows) {
    if (row.type === type && row.key !== '') labels[row.key] = row.value;
  }
  return labels;
}

function labelsForScope(scope: LabelScope, labels: { [key: string]: string }) {
  switch (scope) {
    case 'identifying':
      return { opampIdLabels: labels };
    case 'nonIdentifying':
      return { opampNonIdLabels: labels };
    case 'otelfleet':
      return { otelfleetLabels: labels };
  }
}

function labelRows(labels: { [key: string]: string }, type: MatchType): LabelRow[] {
  return Object.entries(labels).map(([key, value]) => ({ type, key, value }));
}
