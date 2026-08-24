import { create } from '@bufbuild/protobuf';
import { anyPack, anyUnpack, type Any } from '@bufbuild/protobuf/wkt';

import {
  KeyPairFilterSchema,
  LabelFilterSchema,
  LabelType,
  MatchType,
  type LabelFilter,
} from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import {
  RouteSchema,
  RouterSchema,
  type Route,
  type Router,
} from '../gen/api/pkg/api/route/v1alpha1/route_pb';

export const ROUTER_TYPE_URL = `type.googleapis.com/${RouterSchema.typeName}`;

// The router is a singleton resource.
export const ROUTER_KEY = 'global';

export interface LabelRow {
  matchType: MatchType;
  key: string;
  value: string;
}

export interface LabelFilterValues {
  type: LabelType;
  labels: LabelRow[];
}

export interface RouteValues {
  name: string;
  configRef: string;
  use: string;
  filters: LabelFilterValues[];
  routes: RouteValues[];
}

export interface RouterValues {
  configRef: string;
  root: RouteValues | null;
  defs: RouteValues[];
}

export const MATCH_TYPE_OPTIONS = [
  { value: String(MatchType.EQ), label: '==' },
  { value: String(MatchType.NEQ), label: '!=' },
  { value: String(MatchType.RE), label: '~=' },
  { value: String(MatchType.NRE), label: '!~' },
];

export const LABEL_TYPE_OPTIONS = [
  { value: String(LabelType.LabelTypeIdentifying), label: 'Collector identity' },
  { value: String(LabelType.LabelTypeNonIdentifying), label: 'Collector environment' },
  { value: String(LabelType.LabelTypeOtelfleet), label: 'User-defined' },
];

export const LABEL_TYPE_DESCRIPTIONS: Record<LabelType, string> = {
  [LabelType.LabelTypeIdentifying]: 'Reported by the collector, the OpenTelemetryCollector information',
  [LabelType.LabelTypeNonIdentifying]: 'Reported by the collector: the OpenTelemetryCollector host information',
  [LabelType.LabelTypeOtelfleet]: 'Labels you assign in OtelFleet',
};

// Defaults set by opampextension's createAgentDescription; agents may report more.
export const LABEL_KEY_SUGGESTIONS: Record<LabelType, { key: string; description: string }[]> = {
  [LabelType.LabelTypeIdentifying]: [
    { key: 'service.instance.id', description: 'Unique ID of the collector instance' },
    { key: 'service.name', description: 'Collector distribution' },
    { key: 'service.version', description: 'Version of the collector build' },
  ],
  [LabelType.LabelTypeNonIdentifying]: [
    { key: 'host.arch', description: 'CPU architecture of the host, e.g. amd64' },
    { key: 'host.name', description: 'Hostname of the machine running the collector' },
    { key: 'os.description', description: 'Human-readable OS version, e.g. Alpine 3.24.1' },
    { key: 'os.type', description: 'Operating system, e.g. linux' },
  ],
  [LabelType.LabelTypeOtelfleet]: [],
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

export function emptyLabelRow(): LabelRow {
  return { matchType: MatchType.EQ, key: '', value: '' };
}

export function emptyLabelFilterValues(): LabelFilterValues {
  return { type: LabelType.LabelTypeIdentifying, labels: [emptyLabelRow()] };
}

export function emptyRouteValues(name = ''): RouteValues {
  return { name, configRef: '', use: '', filters: [], routes: [] };
}

export function emptyRouterValues(): RouterValues {
  return { configRef: '', root: null, defs: [] };
}

export function toRouter(values: RouterValues): Router {
  return create(RouterSchema, {
    configRef: values.configRef,
    root: values.root ? toRoute(values.root) : undefined,
    defs: values.defs.map(toRoute),
  });
}

export function packRouter(values: RouterValues): Any {
  return anyPack(RouterSchema, toRouter(values));
}

export function unpackRouter(obj?: Any): Router | undefined {
  if (!obj) return undefined;
  return anyUnpack(obj, RouterSchema);
}

export function toRouterValues(pb?: Router): RouterValues {
  if (!pb) return emptyRouterValues();
  return {
    configRef: pb.configRef,
    root: pb.root ? toRouteValues(pb.root) : null,
    defs: pb.defs.map(toRouteValues),
  };
}

function toRoute(values: RouteValues): Route {
  return create(RouteSchema, {
    name: values.name,
    configRef: values.configRef,
    use: values.use,
    filters: values.filters.map(toLabelFilter),
    routes: values.routes.map(toRoute),
  });
}

function toRouteValues(pb: Route): RouteValues {
  return {
    name: pb.name,
    configRef: pb.configRef,
    use: pb.use,
    filters: pb.filters.map(toLabelFilterValues),
    routes: pb.routes.map(toRouteValues),
  };
}

function toLabelFilter(values: LabelFilterValues): LabelFilter {
  return create(LabelFilterSchema, {
    type: values.type,
    filters: values.labels
      .filter((row) => row.key !== '')
      .map((row) =>
        create(KeyPairFilterSchema, {
          matchType: row.matchType,
          label: { key: row.key, value: row.value },
        }),
      ),
  });
}

function toLabelFilterValues(pb: LabelFilter): LabelFilterValues {
  return {
    type: pb.type,
    labels: pb.filters.map((f) => ({
      matchType: f.matchType,
      key: f.label?.key ?? '',
      value: f.label?.value ?? '',
    })),
  };
}

export type AssignmentChangeKind = 'unchanged' | 'changed' | 'assigned' | 'unassigned';

export interface AssignmentChange {
  collector: string;
  oldConfigRef: string;
  newConfigRef: string;
  kind: AssignmentChangeKind;
}

export function diffAssignments(
  oldRefs: { [key: string]: string },
  newRefs: { [key: string]: string },
): AssignmentChange[] {
  const collectors = [...new Set([...Object.keys(oldRefs), ...Object.keys(newRefs)])].sort();
  return collectors.map((collector) => {
    const oldConfigRef = oldRefs[collector] ?? '';
    const newConfigRef = newRefs[collector] ?? '';
    return { collector, oldConfigRef, newConfigRef, kind: changeKind(oldConfigRef, newConfigRef) };
  });
}

function changeKind(oldConfigRef: string, newConfigRef: string): AssignmentChangeKind {
  if (oldConfigRef === newConfigRef) return 'unchanged';
  if (oldConfigRef === '') return 'assigned';
  if (newConfigRef === '') return 'unassigned';
  return 'changed';
}
