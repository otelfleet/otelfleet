import { useEffect, useMemo } from "react";
import type { Node } from "reactflow"
import ReactFlow, { ReactFlowProvider, useReactFlow, useNodesState, useEdgesState } from "reactflow";
import YAML from "yaml";
import type { OTELConfig } from "./types";
import { useClientNodes, useEdgeCreator } from "./layout";

const EmptyStateNodeData: Node[] = [
    { id: "empty-node", type: "emptyState", position: { x: 0, y: 0 }, data: { value: "" } },
];

export default function Flow({
    value,
}: {
    value: string;
}){
    return (
        <ReactFlowProvider>
            <FlowInner value={value}></FlowInner>
        </ReactFlowProvider>
    )
}

function FlowInner({
    value,
}: {
    value: string;
}) {
    const reactFlowInstance = useReactFlow();
    const jsonData = useMemo(() => {
        try {
            return YAML.parse(value, { logLevel: "error", schema: "failsafe" }) as OTELConfig;
        } catch {
            return undefined;
        }
    }, [value]) as OTELConfig;

    const initNodes = useClientNodes(jsonData);
    const initEdges = useEdgeCreator(initNodes ?? []);
    const [nodes, setNodes] = useNodesState(initNodes !== undefined ? initNodes : []);
    const [edges, setEdges] = useEdgesState(initEdges);

    useEffect(() => {
        if (jsonData) {
            setEdges(initEdges);
            setNodes(initNodes !== undefined ? initNodes : []);
            reactFlowInstance.fitView();
        } else {
            setNodes(EmptyStateNodeData);
            setEdges([]);
            reactFlowInstance.fitView();
        }
    }, [initNodes, initEdges, value, jsonData, setEdges, setNodes, reactFlowInstance]);

    return (
        <ReactFlow
            nodes={jsonData?  nodes : EmptyStateNodeData}
            edges={edges}
        >
        </ReactFlow>
    )
}

