import { memo } from 'react';
import { pipelineConfigs, type ParentNodeData } from './types';
import { TracesIcon, MetricsIcon, LogsIcon } from './Icons';

interface ParentNodeProps {
    data: ParentNodeData;
}

function getPipelineIcon(label: string) {
    if (/^traces/i.test(label)) return <TracesIcon />;
    if (/^metrics/i.test(label)) return <MetricsIcon />;
    if (/^logs/i.test(label)) return <LogsIcon />;
    if (/^spans/i.test(label)) return <TracesIcon />;
    return <TracesIcon />;
}

function ParentNode({ data }: ParentNodeProps) {
    const config = pipelineConfigs.find((c) => data.label.match(c.typeRegex));

    if (!config) {
        // Fallback for unknown pipeline types
        return (
            <div
                style={{
                    backgroundColor: 'rgba(156, 163, 175, 0.08)',
                    border: '1px dashed #6B7280',
                    height: data.height,
                    width: data.width,
                    borderRadius: 4,
                    position: 'relative',
                }}
            >
                <div
                    style={{
                        position: 'absolute',
                        top: -10,
                        left: 12,
                        backgroundColor: '#6B7280',
                        color: '#fff',
                        padding: '2px 8px',
                        borderRadius: 4,
                        fontSize: 10,
                        fontWeight: 500,
                        display: 'flex',
                        alignItems: 'center',
                        gap: 4,
                    }}
                >
                    {getPipelineIcon(data.label)}
                    {data.label}
                </div>
            </div>
        );
    }

    return (
        <div
            style={{
                backgroundColor: config.backgroundColor,
                border: config.borderColor,
                height: data.height,
                width: data.width,
                borderRadius: 4,
                position: 'relative',
            }}
        >
            <div
                style={{
                    position: 'absolute',
                    top: -10,
                    left: 12,
                    backgroundColor: config.tagBackgroundColor,
                    color: config.type === 'traces' ? '#78350f' : config.type === 'logs' ? '#064e3b' : '#fff',
                    padding: '2px 8px',
                    borderRadius: 4,
                    fontSize: 10,
                    fontWeight: 500,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 4,
                }}
            >
                {getPipelineIcon(data.label)}
                {data.label}
            </div>
        </div>
    );
}

export default memo(ParentNode);
