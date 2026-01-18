import { memo, useState } from 'react';
import { Handle, Position } from 'reactflow';
import type { NodeData } from './types';

const handleStyle = {
    backgroundColor: 'transparent',
    borderColor: 'transparent',
};

interface BaseNodeProps {
    data: NodeData;
    icon: React.ReactNode;
    nodeType: 'receiver' | 'processor' | 'exporter';
    sourceHandle?: boolean;
    targetHandle?: boolean;
}

function BaseNode({ data, icon, nodeType, sourceHandle = true, targetHandle = true }: BaseNodeProps) {
    const [hovered, setHovered] = useState(false);

    const isConnector = data.type.includes('connectors');
    const label = data.label || '';
    const splitLabel = label.includes('/') ? label.split('/') : [label];

    const getHeaderColor = () => {
        if (isConnector) return '#22c55e'; // green for connectors
        if (nodeType === 'processor') return '#3b82f6'; // blue
        return '#8b5cf6'; // violet for receivers/exporters
    };

    const headerColor = getHeaderColor();

    return (
        <div
            style={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                width: 120,
                height: 80,
                borderRadius: 8,
                boxShadow: '0 2px 4px rgba(0,0,0,0.2)',
                marginLeft: nodeType === 'exporter' ? 12 : nodeType === 'receiver' ? 0 : 12,
                marginRight: nodeType === 'receiver' ? 12 : 0,
            }}
            onMouseEnter={() => setHovered(true)}
            onMouseLeave={() => setHovered(false)}
        >
            {/* Header */}
            <div
                style={{
                    borderRadius: '8px 8px 0 0',
                    backgroundColor: headerColor,
                    color: isConnector ? '#14532d' : '#fff',
                    fontSize: 12,
                    fontWeight: 500,
                    height: '35%',
                    width: '100%',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    overflow: 'hidden',
                    whiteSpace: 'nowrap',
                    textOverflow: 'ellipsis',
                    padding: '0 8px',
                }}
            >
                {splitLabel[0]}
            </div>

            {/* Body */}
            <div
                style={{
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: '100%',
                    height: '65%',
                    background: hovered ? '#40454E' : '#30353D',
                    transition: 'background-color 0.2s ease',
                    borderRadius: '0 0 8px 8px',
                    borderLeft: `1px solid ${hovered ? '#6D737D' : '#40454E'}`,
                    borderRight: `1px solid ${hovered ? '#6D737D' : '#40454E'}`,
                    borderBottom: `1px solid ${hovered ? '#6D737D' : '#40454E'}`,
                    cursor: 'pointer',
                    position: 'relative',
                }}
            >
                {targetHandle && (
                    <Handle
                        type="target"
                        position={Position.Left}
                        style={handleStyle}
                    />
                )}

                <div style={{ color: hovered ? '#F3F5F6' : '#9CA2AB', transition: 'color 0.2s ease' }}>
                    {icon}
                </div>

                {splitLabel.length > 1 && (
                    <div
                        style={{
                            color: hovered ? '#9CA2AB' : '#6D737D',
                            fontSize: 10,
                            overflow: 'hidden',
                            whiteSpace: 'nowrap',
                            textOverflow: 'ellipsis',
                            maxWidth: '90%',
                            marginTop: 2,
                        }}
                    >
                        {splitLabel[1]}
                    </div>
                )}

                {sourceHandle && (
                    <Handle
                        type="source"
                        position={Position.Right}
                        style={handleStyle}
                    />
                )}
            </div>
        </div>
    );
}

export default memo(BaseNode);
