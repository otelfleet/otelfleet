import { useMemo } from "react";
import type { OTELConfig, OTELPipeline } from "./types";
import {type Node, type XYPosition, type Edge, MarkerType} from "reactflow"
const childNodesHeight = 80;



export const useClientNodes = (value: OTELConfig) => {
    return useMemo(() => calcNodes(value), [value]);
};

const createNode = (pipelineName: string, parentNode: OTELPipeline, height: number, connectors?: object) => {
	const nodesToAdd: Node[] = [];
	const keyTraces = Object.keys(parentNode);

	const calcYPosition = (index: number, parentHeight: number, nodes: string[]): number | undefined => {
		const childNodePositions = [];
		const spaceBetweenNodes = (parentHeight - nodes.length * childNodesHeight) / (nodes.length + 1);

		for (let i = 0; i < nodes.length; i++) {
			const yPosition = spaceBetweenNodes + i * (childNodesHeight + spaceBetweenNodes);

			childNodePositions.push(yPosition);
		}
		switch (nodes.length) {
			case 0:
				return;
			case 1:
				return (parentHeight - 40) / 2 - 20;
			default:
				return childNodePositions[index];
		}
	};

	const processorPosition = (index: number, parentHeight: number, receivers: string[]): XYPosition => {
		const receiverLength = receivers.length ? 250 : 0;
		return { x: receiverLength + index * 200, y: (parentHeight - 40) / 2 - 20 };
	};

	const receiverPosition = (index: number, parentHeight: number, receivers: string[]): XYPosition => {
		const positionY = calcYPosition(index, parentHeight, receivers);
		return { x: 50, y: positionY ?? parentHeight / 2 };
	};

	const exporterPosition = (
		index: number,
		parentHeight: number,
		exporters: string[],
		processors: string[]
	): XYPosition => {
		const positionY = calcYPosition(index, parentHeight, exporters);
		const processorLength = (processors?.length ?? 0) * 200 + 260;
		return { x: processorLength, y: positionY ?? parentHeight / 2 };
	};
	const processors = parentNode.processors;
	const receivers = parentNode.receivers;
	const exporters = parentNode.exporters;
	keyTraces.forEach((traceItem) => {
		switch (traceItem) {
			case "processors":
				processors?.map((processor, index) => {
					const id = `${pipelineName}-Processor-processorNode-${processor}`;

					nodesToAdd.push({
						id: id,
						parentNode: pipelineName,
						extent: "parent",
						type: "processorsNode",
						position: processorPosition(index, height, processors),
						data: {
							label: processor,
							parentNode: pipelineName,
							type: "processors",
							height: childNodesHeight,
							id: id,
							position: processorPosition(index, height, processors),
						},
						draggable: false,
					});
				});
				break;
			case "receivers":
				receivers?.map((receiver, index) => {
					const isConnector = connectors?.hasOwnProperty(receiver) ? "connectors/receivers" : "receivers";
					const id = `${pipelineName}-Receiver-receiverNode-${receiver}`;

					nodesToAdd.push({
						id: id,
						parentNode: pipelineName,
						extent: "parent",
						type: "receiversNode",
						position: receiverPosition(index, height, receivers),
						data: {
							label: receiver,
							parentNode: pipelineName,
							type: isConnector,
							height: childNodesHeight,
							id: id,
							position: receiverPosition(index, height, receivers),
						},
						draggable: false,
					});
				});
				break;
			case "exporters":
				exporters?.map((exporter, index) => {
					const isConnector = connectors?.hasOwnProperty(exporter) ? "connectors/exporters" : "exporters";
					const id = `${pipelineName}-exporter-exporterNode-${exporter}`;
					nodesToAdd.push({
						id: id,
						parentNode: pipelineName,
						extent: "parent",
						type: "exportersNode",
						position: exporterPosition(index, height, exporters, processors ?? []),
						data: {
							label: exporter,
							parentNode: pipelineName,
							type: isConnector,
							height: childNodesHeight,
							id: id,
							position: exporterPosition(index, height, exporters, processors ?? []),
						},
						draggable: false,
					});
				});
				break;
		}
	});
	return nodesToAdd;
};

export const calcNodes = (value: OTELConfig) => {
    const pipelines = value?.service?.pipelines;
    const connectors = value?.connectors;
    if (pipelines == null) {
        return [];
    }

    const nodesToAdd: Node[] = [];
    const gapBetweenPipelines = 60;
    let currentY = 0;

    for (const [pipelineName, pipeline] of Object.entries(pipelines)) {
        const receivers = pipeline.receivers?.length ?? 0;
        const exporters = pipeline.exporters?.length ?? 0;
        const maxNodes = Math.max(receivers, exporters, 1);
        const spaceBetweenParents = 40;
        const spaceBetweenNodes = 90;
        const totalSpacing = maxNodes * spaceBetweenNodes;
        const parentHeight = totalSpacing + maxNodes * childNodesHeight;
        const actualHeight = maxNodes === 1 ? parentHeight : parentHeight + spaceBetweenParents;

        nodesToAdd.push({
            id: pipelineName,
            type: "parentNodeType",
            position: { x: 0, y: currentY },
            data: {
                label: pipelineName,
                parentNode: pipelineName,
                width: 430 + 200 * (pipeline.processors?.length ?? 0),
                height: actualHeight,
                type: "parentNodeType",
                childNodes: createNode(pipelineName, pipeline, actualHeight, connectors),
            },
            draggable: false,
            ariaLabel: pipelineName,
            expandParent: true,
        });
        const childNodes = createNode(pipelineName, pipeline, actualHeight, connectors);
        nodesToAdd.push(...childNodes);

        // Move Y position down for the next pipeline
        currentY += actualHeight + gapBetweenPipelines;
    }
    return nodesToAdd;
};

function createEdge(sourceNode: Node, targetNode: Node): Edge {
	const edgeId = `edge-${sourceNode.id}-${targetNode.id}`;
	return {
		id: edgeId,
		source: sourceNode.id,
		target: targetNode.id,
		type: "default",
		markerEnd: {
			type: MarkerType.Arrow,
			color: "#9CA2AB",
			width: 20,
			height: 25,
		},
		style: {
			stroke: "#9CA2AB",
		},
		data: {
			type: "edge",
			sourceParent: sourceNode.parentNode,
			targetParent: targetNode.parentNode,
		},
	};
}

function createConnectorEdge(sourceNode: Node, targetNode: Node): Edge {
	const edgeId = `edge-${sourceNode.id}-${targetNode.id}`;
	return {
		id: edgeId,
		source: sourceNode.id,
		target: targetNode.id,
		type: "default",
		markerEnd: {
			type: MarkerType.Arrow,
			color: "#9CA2AB",
			width: 20,
			height: 25,
		},
		style: {
			stroke: "#9CA2AB",
		},
		data: {
			type: "connector",
			sourcePipeline: sourceNode.parentNode,
			targetPipeline: targetNode.parentNode,
		},
	};
}


export function calcEdges(nodeIdsArray: Node[]) {
	const edges: Edge[] = [];

	const calculateExportersNode = (exportersNodes: Node[], processorsNode: Node) => {
		if (!processorsNode) {
			return;
		}

		const newEdges = exportersNodes
			.filter((targetNode) => targetNode !== undefined)
			.map((targetNode) => createEdge(processorsNode, targetNode));

		edges.push(...newEdges);
	};

	const calculateProcessorsNode = (processorsNodes: Node[]) => {
		for (let i = 0; i < processorsNodes.length; i++) {
			const sourceNode = processorsNodes[i];
			const targetNode = processorsNodes[i + 1];
			if (!sourceNode || !targetNode) {
				continue;
			}
			const edge = createEdge(sourceNode, targetNode);
			edges.push(edge);
		}
	};

	const calculateReceiversNode = (
		receiversNodes: Node[],
		firstProcessorsNode: Node | undefined,
		exportersNodes: Node[]
	) => {
		const processNode = (sourceNode: Node, targetNode: Node) => {
			if (!sourceNode || !targetNode) {
				return;
			}

			const edge = createEdge(sourceNode, targetNode);
			edges.push(edge);
		};

		if (!firstProcessorsNode) {
			receiversNodes.forEach((sourceNode) => {
				exportersNodes.forEach((exporterNode) => processNode(sourceNode, exporterNode));
			});
		} else {
			receiversNodes.forEach((sourceNode) => processNode(sourceNode, firstProcessorsNode));
		}
	};

	const calculateConnectorsNode = (nodes: Node[]) => {
		const connectorsAsExporter = nodes.filter((node) => node?.data?.type === "connectors/exporters");
		const connectorsAsReceiver = nodes.filter((node) => node?.data?.type === "connectors/receivers");

		const connectorEdges = connectorsAsExporter.flatMap((sourceNode) =>
			connectorsAsReceiver
				.filter((node) => node?.data?.label === sourceNode?.data?.label)
				.map((targetNode) => createConnectorEdge(sourceNode, targetNode))
		);

		edges.push(...connectorEdges);
	};

	const addEdgesToNodes = (nodes: Node[]) => {
		const exportersNodes = nodes.filter((node) => node.type === "exportersNode");
		const processorsNodes = nodes.filter((node) => node.type === "processorsNode");
		const receiversNodes = nodes.filter((node) => node.type === "receiversNode");
		const firstProcessorsNode = processorsNodes[0] as Node;
		const lastProcessorsNode = processorsNodes[processorsNodes.length - 1] as Node;

		calculateExportersNode(exportersNodes, lastProcessorsNode);
		calculateProcessorsNode(processorsNodes);
		calculateReceiversNode(receiversNodes, firstProcessorsNode, exportersNodes);
	};

	const childNodes = (parentNode: string) => {
		return nodeIdsArray.filter((node) => node.parentNode === parentNode);
	};

	const parentNodes = nodeIdsArray.filter((node) => node.type === "parentNodeType").map((node) => node.data.label);
	if (!Array.isArray(nodeIdsArray) || nodeIdsArray.length < 2) {
		return [];
	}

	parentNodes.forEach((parentNode) => {
		const childNode = childNodes(parentNode);
		addEdgesToNodes(childNode);
	});

	calculateConnectorsNode(
		nodeIdsArray.filter((node) => node.type === "exportersNode" || node.type === "receiversNode")
	);

	return edges;
}

export function useEdgeCreator(nodeIdsArray: Node[]) {
	return useMemo(() => {
		return calcEdges(nodeIdsArray);
	}, [nodeIdsArray]);
}