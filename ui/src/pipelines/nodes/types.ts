export interface NodeData {
    label: string;
    parentNode: string;
    type: 'receivers' | 'processors' | 'exporters' | 'connectors/receivers' | 'connectors/exporters' | 'parentNodeType';
    height: number;
    id: string;
    position: { x: number; y: number };
    width?: number;
}

export interface ParentNodeData {
    label: string;
    parentNode: string;
    width: number;
    height: number;
    type: 'parentNodeType';
}

export type PipelineType = 'traces' | 'metrics' | 'logs' | 'spans';

export interface PipelineConfig {
    type: PipelineType;
    typeRegex: RegExp;
    backgroundColor: string;
    tagBackgroundColor: string;
    borderColor: string;
}

export const pipelineConfigs: PipelineConfig[] = [
    {
        type: 'traces',
        typeRegex: /^traces(\/.*)?$/i,
        backgroundColor: 'rgba(251, 191, 36, 0.08)',
        tagBackgroundColor: '#FBBF24',
        borderColor: '1px dashed #F59E0B',
    },
    {
        type: 'metrics',
        typeRegex: /^metrics(\/.*)?$/i,
        backgroundColor: 'rgba(56, 189, 248, 0.08)',
        tagBackgroundColor: '#38BDF8',
        borderColor: '1px dashed #0AA8FF',
    },
    {
        type: 'logs',
        typeRegex: /^logs(\/.*)?$/i,
        backgroundColor: 'rgba(52, 211, 153, 0.08)',
        tagBackgroundColor: '#34D399',
        borderColor: '1px dashed #40ad54',
    },
    {
        type: 'spans',
        typeRegex: /^spans(\/.*)?$/i,
        backgroundColor: 'rgba(145, 29, 201, 0.08)',
        tagBackgroundColor: '#911dc9',
        borderColor: '1px dashed #911dc9',
    },
];
