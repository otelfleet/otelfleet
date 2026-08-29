import { MarkerType, type Edge, type Node } from 'reactflow';

import { LabelType, MatchType } from '../gen/api/pkg/api/common/v1alpha1/common_pb';
import type { LabelFilterValues, RouteValues, RouterValues } from './router';

export interface RouteNodeData {
  name: string;
  configRef: string;
  use: string;
  matchers: string[];
  collectors: number;
  isRoot: boolean;
  isDef: boolean;
  isDefault: boolean;
  selected: boolean;
  matched: boolean;
}

const ARROW = { type: MarkerType.ArrowClosed, width: 18, height: 18 };

const EDGE_LABEL = {
  labelStyle: { fill: 'var(--mantine-color-dimmed)', fontSize: 11 },
  labelBgStyle: { fill: 'var(--mantine-color-body)' },
  labelBgPadding: [6, 2] as [number, number],
  labelBgBorderRadius: 4,
};

const NODE_WIDTH = 260;
const NODE_HEIGHT = 120;
const COLUMN_GAP = 80;
const ROW_GAP = 24;

const MATCH_TYPE_SYMBOLS: Record<MatchType, string> = {
  [MatchType.EQ]: '==',
  [MatchType.NEQ]: '!=',
  [MatchType.RE]: '=~',
  [MatchType.NRE]: '!~',
};

export const LABEL_TYPE_NAMES: Record<LabelType, string> = {
  [LabelType.LabelTypeIdentifying]: 'identity',
  [LabelType.LabelTypeNonIdentifying]: 'environment',
  [LabelType.LabelTypeOtelfleet]: 'user-defined',
};

export function describeLabelFilter(filter: LabelFilterValues): string[] {
  const scope = LABEL_TYPE_NAMES[filter.type];
  return filter.labels
    .filter((row) => row.key !== '')
    .map((row) => `${scope} | ${row.key} ${MATCH_TYPE_SYMBOLS[row.matchType]} ${row.value}`);
}

export const ROOT_NODE_ID = 'root';

export const DEFAULT_NODE_ID = 'default';

export function nodeIdFromIndexPath(indexPath: number[]): string {
  return [ROOT_NODE_ID, ...indexPath].join('-');
}

export interface RouteGraph {
  nodes: Node<RouteNodeData>[];
  edges: Edge[];
}

// Collectors are attributed to a route by the config ref it assigns, so routes
// sharing a config ref share a count.
export function buildRouteGraph(
  values: RouterValues,
  assignedConfigRefs: string[],
): RouteGraph {
  const counts = countByConfigRef(assignedConfigRefs);
  const nodes: Node<RouteNodeData>[] = [];
  const edges: Edge[] = [];
  const cursor = { y: 0 };

  const rootY = values.root
    ? addTree(values.root, ROOT_NODE_ID, 1, { nodes, edges, counts, cursor, isDef: false })
    : nextRow(cursor);

  nodes.push(defaultNode(values.configRef, counts[values.configRef] ?? 0, rootY));
  if (values.root) {
    edges.push({
      id: `${DEFAULT_NODE_ID}-routes`,
      source: DEFAULT_NODE_ID,
      target: ROOT_NODE_ID,
      markerEnd: ARROW,
      label: 'routed by',
      ...EDGE_LABEL,
    });
  }

  for (const [index, def] of values.defs.entries()) {
    cursor.y += NODE_HEIGHT + ROW_GAP;
    addTree(def, `def-${index}`, 1, {
      nodes, edges, counts, cursor, isDef: true,
    });
  }

  for (const node of nodes) {
    if (node.data.use === '') continue;
    const target = nodes.find((candidate) => candidate.data.isDef && candidate.data.name === node.data.use);
    if (target) {
      edges.push({
        id: `${node.id}-uses-${target.id}`,
        source: node.id,
        target: target.id,
        animated: true,
        style: { strokeDasharray: '4 4' },
        markerEnd: ARROW,
        label: 'use',
        ...EDGE_LABEL,
      });
    }
  }

  return { nodes, edges };
}

function defaultNode(configRef: string, collectors: number, y: number): Node<RouteNodeData> {
  return {
    id: DEFAULT_NODE_ID,
    type: 'route',
    position: { x: 0, y },
    data: {
      name: 'default config',
      configRef,
      use: '',
      matchers: [],
      collectors,
      isRoot: false,
      isDef: false,
      isDefault: true,
      selected: false,
      matched: false,
    },
  };
}

interface TreeContext {
  nodes: Node<RouteNodeData>[];
  edges: Edge[];
  counts: Record<string, number>;
  cursor: { y: number };
  isDef: boolean;
}

function addTree(route: RouteValues, id: string, depth: number, ctx: TreeContext): number {
  const isRoot = depth === 1 && !ctx.isDef;
  const childCenters = route.routes.map((child, index) =>
    addTree(child, `${id}-${index}`, depth + 1, ctx),
  );

  const y = childCenters.length > 0
    ? (childCenters[0] + childCenters[childCenters.length - 1]) / 2
    : nextRow(ctx.cursor);

  ctx.nodes.push({
    id,
    type: 'route',
    position: { x: depth * (NODE_WIDTH + COLUMN_GAP), y },
    data: {
      name: route.name || '(unnamed)',
      configRef: route.configRef,
      use: route.use,
      matchers: route.filters.flatMap(describeLabelFilter),
      collectors: ctx.counts[route.configRef] ?? 0,
      isRoot,
      isDef: depth === 1 && ctx.isDef,
      isDefault: false,
      selected: false,
      matched: false,
    },
  });

  for (const [index] of route.routes.entries()) {
    ctx.edges.push({
      id: `${id}-${index}-edge`,
      source: id,
      target: `${id}-${index}`,
      markerEnd: ARROW,
    });
  }

  return y;
}

function nextRow(cursor: { y: number }): number {
  const y = cursor.y;
  cursor.y += NODE_HEIGHT + ROW_GAP;
  return y;
}

function countByConfigRef(assignedConfigRefs: string[]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const configRef of assignedConfigRefs) {
    if (configRef === '') continue;
    counts[configRef] = (counts[configRef] ?? 0) + 1;
  }
  return counts;
}
