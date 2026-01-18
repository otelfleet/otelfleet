import { memo } from 'react';
import BaseNode from './BaseNode';
import { ReceiverIcon, ConnectorIcon } from './Icons';
import type { NodeData } from './types';

interface ReceiverNodeProps {
    data: NodeData;
}

function ReceiverNode({ data }: ReceiverNodeProps) {
    const isConnector = data.type.includes('connectors');

    return (
        <BaseNode
            data={data}
            icon={isConnector ? <ConnectorIcon /> : <ReceiverIcon />}
            nodeType="receiver"
            targetHandle={true}
            sourceHandle={true}
        />
    );
}

export default memo(ReceiverNode);
