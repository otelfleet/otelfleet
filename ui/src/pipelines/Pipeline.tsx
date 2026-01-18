import { useEffect, useMemo, useCallback } from "react";
import type { Node } from "reactflow"
import ReactFlow, {
    ReactFlowProvider,
    useReactFlow,
    useNodesState,
    useEdgesState,
    Controls,
    Background,
    BackgroundVariant,
} from "reactflow";
import YAML from "yaml";
import type { OTELConfig } from "./types";
import { useClientNodes, useEdgeCreator } from "./layout";
import {
    ReceiverNode,
    ProcessorNode,
    ExporterNode,
    ParentNode,
    EmptyStateNode,
} from "./nodes";

import "reactflow/dist/style.css";

const nodeTypes = {
    receiversNode: ReceiverNode,
    processorsNode: ProcessorNode,
    exportersNode: ExporterNode,
    parentNodeType: ParentNode,
    emptyState: EmptyStateNode,
};

const EmptyStateNodeData: Node[] = [
    { id: "empty-node", type: "emptyState", position: { x: 0, y: 0 }, data: { value: "" } },
];

interface PipelineGraphProps {
    value: string;
}

export default function PipelineGraph({ value }: PipelineGraphProps) {
    return (
        <ReactFlowProvider>
            <PipelineGraphInner value={value} />
        </ReactFlowProvider>
    );
}

function PipelineGraphInner({ value }: PipelineGraphProps) {
    const reactFlowInstance = useReactFlow();

    const jsonData = useMemo(() => {
        try {
            return YAML.parse(value, { logLevel: "error", schema: "failsafe" }) as OTELConfig;
        } catch {
            return undefined;
        }
    }, [value]);

    const initNodes = useClientNodes(jsonData as OTELConfig);
    const initEdges = useEdgeCreator(initNodes ?? []);
    const [nodes, setNodes] = useNodesState(initNodes !== undefined ? initNodes : []);
    const [edges, setEdges] = useEdgesState(initEdges);

    useEffect(() => {
        if (jsonData) {
            setEdges(initEdges);
            setNodes(initNodes !== undefined ? initNodes : []);
            // Small delay to ensure nodes are rendered before fitting view
            setTimeout(() => {
                reactFlowInstance.fitView({ padding: 0.2 });
            }, 50);
        } else {
            setNodes(EmptyStateNodeData);
            setEdges([]);
            setTimeout(() => {
                reactFlowInstance.fitView({ padding: 0.2 });
            }, 50);
        }
    }, [initNodes, initEdges, jsonData, setEdges, setNodes, reactFlowInstance]);

    const onInit = useCallback(() => {
        reactFlowInstance.fitView({ padding: 0.2 });
    }, [reactFlowInstance]);

    return (
        <ReactFlow
            nodes={jsonData ? nodes : EmptyStateNodeData}
            edges={edges}
            nodeTypes={nodeTypes}
            onInit={onInit}
            fitView
            fitViewOptions={{ padding: 0.2 }}
            minZoom={0.1}
            maxZoom={2}
            proOptions={{ hideAttribution: true }}
            nodesDraggable={false}
            nodesConnectable={false}
            elementsSelectable={false}
            zoomOnScroll
        >
            <Controls
                showInteractive={false}
                style={{
                    backgroundColor: '#2b2c3d',
                    borderRadius: 4,
                    border: '1px solid #4d4f66',
                }}
            />
            <Background
                variant={BackgroundVariant.Dots}
                gap={20}
                size={1}
                color="#4d4f66"
            />
        </ReactFlow>
    );
}
