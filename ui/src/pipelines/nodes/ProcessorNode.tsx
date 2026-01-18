import { memo } from 'react';
import BaseNode from './BaseNode';
import { ProcessorIcon } from './Icons';
import type { NodeData } from './types';

interface ProcessorNodeProps {
    data: NodeData;
}

function ProcessorNode({ data }: ProcessorNodeProps) {
    return (
        <BaseNode
            data={data}
            icon={<ProcessorIcon />}
            nodeType="processor"
            targetHandle={true}
            sourceHandle={true}
        />
    );
}

export default memo(ProcessorNode);
