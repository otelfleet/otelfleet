import { useCallback, useMemo, type ReactNode } from 'react';
import ReactFlow, { Background, BackgroundVariant, Controls, type Node, type NodeMouseHandler } from 'reactflow';

import RouteNode from './RouteNode';
import { buildRouteGraph, type RouteNodeData } from './routeGraph';
import type { RouterValues } from './router';

import 'reactflow/dist/style.css';

const nodeTypes = { route: RouteNode };

interface RouterGraphProps {
  children?: ReactNode;
  value: RouterValues;
  assignedConfigRefs: string[];
  selectedNodeId?: string;
  matchedNodeId?: string;
  onSelect?: (nodeId: string, data: RouteNodeData) => void;
}

export function RouterGraph({
  children,
  value,
  assignedConfigRefs,
  selectedNodeId,
  matchedNodeId,
  onSelect,
}: RouterGraphProps) {
  const graph = useMemo(() => buildRouteGraph(value, assignedConfigRefs), [value, assignedConfigRefs]);

  const nodes = useMemo(
    () =>
      graph.nodes.map((node) => ({
        ...node,
        data: {
          ...node.data,
          selected: node.id === selectedNodeId,
          matched: node.id === matchedNodeId,
        },
      })),
    [graph.nodes, selectedNodeId, matchedNodeId],
  );
  const edges = graph.edges;

  const handleNodeClick = useCallback<NodeMouseHandler>(
    (_event, node) => onSelect?.(node.id, (node as Node<RouteNodeData>).data),
    [onSelect],
  );

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodeClick={handleNodeClick}
      fitView
      fitViewOptions={{ padding: 0.2, maxZoom: 1 }}
      minZoom={0.1}
      maxZoom={2}
      proOptions={{ hideAttribution: true }}
      nodesDraggable={false}
      nodesConnectable={false}
    >
      {children}
      <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
      <Controls showInteractive={false} />
    </ReactFlow>
  );
}
