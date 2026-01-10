export interface OTELConfig {
    [key: string]: unknown;
	connectors: object;
	service?: OTELService;
}

export interface OTELPipeline {
    exporters?: string[];
    processors?: string[];
    receivers?: string[];
}

export interface OTELPipelines {
    [key:string] : OTELPipeline;
}

export interface OTELService {
    pipelines?: OTELPipelines;
}
