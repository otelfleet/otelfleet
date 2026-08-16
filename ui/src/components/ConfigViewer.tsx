import { useState } from "react";
import MonacoEditor, { type OnMount } from "@monaco-editor/react";
import { Box, Group, Paper, SegmentedControl } from '@mantine/core';
import { useMonacoTheme } from "../hooks/useMonacoTheme";
import PipelineGraph from "../pipelines/Pipeline";

interface ConfigViewerProps {
    config?: string | null;
    height?: number | string;
}

type ViewMode = 'editor' | 'graph' | 'split';

export function ConfigViewer({ config, height }: ConfigViewerProps) {
    const monacoTheme = useMonacoTheme();
    const content = config ?? "";
    const [viewMode, setViewMode] = useState<ViewMode>('split');

    const handleEditorMount: OnMount = (editor) => {
        const node = editor.getContainerDomNode();
        const observer = new ResizeObserver(() => {
            const { width, height } = node.getBoundingClientRect();
            if (width === 0 || height === 0) return;
            editor.layout();
            editor.setScrollTop(0);
            observer.disconnect();
        });
        observer.observe(node);
    }

    const showEditor = viewMode === 'editor' || viewMode === 'split';
    const showGraph = viewMode === 'graph' || viewMode === 'split';

    return (
        <Box
            style={{
                display: "flex",
                flexDirection: "column",
                flex: 1,
                height: height ?? "100%",
                minHeight: 0,
                gap: 16,
            }}
        >
            <Group>
                <SegmentedControl
                    value={viewMode}
                    onChange={(value) => setViewMode(value as ViewMode)}
                    data={[
                        { label: 'Editor', value: 'editor' },
                        { label: 'Split', value: 'split' },
                        { label: 'Graph', value: 'graph' },
                    ]}
                />
            </Group>

            <Box
                style={{
                    flex: 1,
                    minHeight: 0,
                    display: 'flex',
                    gap: 16,
                }}
            >
                {showEditor && (
                    <Paper
                        shadow="sm"
                        radius="md"
                        style={{
                            flex: 1,
                            minHeight: 0,
                            overflow: 'hidden',
                        }}
                    >
                        <MonacoEditor
                            value={content}
                            width="100%"
                            height="100%"
                            defaultLanguage="yaml"
                            theme={monacoTheme}
                            options={{
                                readOnly: true,
                                quickSuggestions: false,
                                automaticLayout: true,
                                scrollBeyondLastLine: false,
                                minimap: { enabled: false },
                                scrollbar: { verticalScrollbarSize: 8, horizontal: "hidden" },
                                padding: { top: 5 },
                                fontSize: 13,
                                fontWeight: "400",
                            }}
                            onMount={handleEditorMount}
                        />
                    </Paper>
                )}

                {showGraph && (
                    <Paper
                        shadow="sm"
                        radius="md"
                        style={{
                            flex: 1,
                            minHeight: 0,
                            overflow: 'hidden',
                            backgroundColor: 'var(--elevation-surface-bg)',
                        }}
                    >
                        <PipelineGraph key={viewMode} value={content} />
                    </Paper>
                )}
            </Box>
        </Box>
    );
}
