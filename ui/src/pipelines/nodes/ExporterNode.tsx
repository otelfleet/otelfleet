import { memo } from 'react';
import BaseNode from './BaseNode';
import { ExporterIcon, ConnectorIcon } from './Icons';
import type { NodeData } from './types';

interface ExporterNodeProps {
    data: NodeData;
}

function ExporterNode({ data }: ExporterNodeProps) {
    const isConnector = data.type.includes('connectors');

    return (
        <BaseNode
            data={data}
            icon={isConnector ? <ConnectorIcon /> : <ExporterIcon />}
            nodeType="exporter"
            targetHandle={true}
            sourceHandle={true}
        />
    );
}

export default memo(ExporterNode);
