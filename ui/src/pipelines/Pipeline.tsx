import React, { useEffect, useMemo } from "react";
import type { Node } from "reactflow"
import ReactFlow, { ReactFlowProvider, Background, Panel, useReactFlow, useNodesState, useEdgesState, useStore } from "reactflow";
import YAML, { Parser } from "yaml";
import type { OTELConfig, OTELService, OTELPipelines, OTELPipeline } from "./types";
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
        } catch (error: unknown) {
            console.log(error)
        }
    }, [value]) as OTELConfig;

    const initNodes = useClientNodes(jsonData);
    const initEdges = useEdgeCreator(initNodes ?? []);
    // const { nodes: layoutedNodes, edges: layoutedEdges } = useLayout(initNodes ?? [], initEdges);
    const [nodes, setNodes] = useNodesState(initNodes !== undefined ? initNodes : []);
    const [edges, setEdges] = useEdgesState(initEdges);
    // useEffect(() => {
    // 	reactFlowInstance.fitView();
    // }, [reactFlowWidth, reactFlowInstance]);

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

