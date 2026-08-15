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
  scope: LabelScope;
  key: string;
  value: string;
}

export interface LabelFilterValues {
  type: MatchType;
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
  { value: String(MatchType.EQ), label: 'equals' },
  { value: String(MatchType.NEQ), label: 'not equals' },
  { value: String(MatchType.RE), label: 'matches regex' },
  { value: String(MatchType.NR), label: 'does not match regex' },
];

export const LABEL_SCOPE_OPTIONS: { value: LabelScope; label: string }[] = [
  { value: 'identifying', label: 'OpAMP identifying' },
  { value: 'nonIdentifying', label: 'OpAMP non-identifying' },
  { value: 'otelfleet', label: 'OtelFleet' },
];

export function emptyConfigFilterValues(): ConfigFilterValues {
  return {
    name: '',
    isDefault: false,
    requiresApproval: false,
    configRef: '',
    filters: [emptyLabelFilterValues()],
  };
}

export function emptyLabelFilterValues(): LabelFilterValues {
  return { type: MatchType.EQ, labels: [{ scope: 'identifying', key: '', value: '' }] };
}

export function packConfigFilter(values: ConfigFilterValues): Any {
  const filters = values.filters.map((filter) =>
    create(LabelFilterSchema, {
      type: filter.type,
      opampIdLabels: labelsForScope(filter.labels, 'identifying'),
      opampNonIdLabels: labelsForScope(filter.labels, 'nonIdentifying'),
      otelfleetLabels: labelsForScope(filter.labels, 'otelfleet'),
    }),
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

  const filters = filter.filters.map((labelFilter) => ({
    type: labelFilter.type,
    labels: [
      ...labelRows(labelFilter.opampIdLabels, 'identifying'),
      ...labelRows(labelFilter.opampNonIdLabels, 'nonIdentifying'),
      ...labelRows(labelFilter.otelfleetLabels, 'otelfleet'),
    ],
  }));

  return {
    name,
    isDefault: filter.default ?? false,
    requiresApproval: filter.approval?.requiresApproval ?? false,
    configRef: filter.collectorConfig?.configRef ?? '',
    filters: filters.length > 0 ? filters : [emptyLabelFilterValues()],
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

function labelsForScope(rows: LabelRow[], scope: LabelScope): { [key: string]: string } {
  const labels: { [key: string]: string } = {};
  for (const row of rows) {
    if (row.scope === scope && row.key !== '') labels[row.key] = row.value;
  }
  return labels;
}

function labelRows(labels: { [key: string]: string }, scope: LabelScope): LabelRow[] {
  return Object.entries(labels).map(([key, value]) => ({ scope, key, value }));
}
