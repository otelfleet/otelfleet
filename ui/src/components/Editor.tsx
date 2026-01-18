import { useState } from "react";
import MonacoEditor, { type OnChange, type OnMount } from "@monaco-editor/react";
import { Box, Button, Group, TextInput, Paper, SegmentedControl } from '@mantine/core';
import { useForm } from '@mantine/form'
import { useClient } from "../api";
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import { notifications } from "@mantine/notifications";
import { notifyGRPCError } from "../api/notifications";
import { useNavigate } from '@tanstack/react-router';
import { useMonacoTheme } from "../hooks/useMonacoTheme";
import { CheckCircledIcon } from '@radix-ui/react-icons';
import PipelineGraph from "../pipelines/Pipeline";

interface EditorProps {
    defaultConfig?: string | null;
    configId?: string;
    readOnly?: boolean;
    height?: number | string;
}

type ViewMode = 'editor' | 'graph' | 'split';

export function Editor({ defaultConfig, configId, readOnly = false, height }: EditorProps) {
    const isEditMode = Boolean(configId);

    const monacoTheme = useMonacoTheme();
    const actualConfig = defaultConfig ? defaultConfig : ""
    const [configData, setConfig] = useState(actualConfig)
    const [viewMode, setViewMode] = useState<ViewMode>('split');

    const handleEditorChange: OnChange = (value) => {
        if (value !== undefined && !readOnly) {
            setConfig(value);
        }
    }

    const handleEditorMount: OnMount = (editor) => {
        requestAnimationFrame(() => {
            editor.layout();
        });
    }

    const form = useForm({
        mode: 'controlled',
        initialValues: {
            configName: configId ?? '',
        },
        validate: {
            configName: (value) => (/[a-zA-Z0-9]/.test(value) ? null : 'Invalid config name'),
        },
    });

    const otelConfigClient = useClient(ConfigService)
    const navigate = useNavigate();

    const handleSubmit = async (values: { configName: string }) => {
        if (readOnly) return;
        try {
            const bytes = new TextEncoder().encode(configData);
            await otelConfigClient.putConfig({
                ref: {
                    id: values.configName,
                },
                config: {
                    config: bytes,
                },
            })
            notifications.show({
                title: isEditMode ? "Config updated" : "Config created",
                message: isEditMode
                    ? `Updated config: ${values.configName}`
                    : `Created config: ${values.configName}`,
                icon: <CheckCircledIcon />,
            })
            navigate({
                to: '/configs',
            });
        } catch (error) {
            notifyGRPCError(isEditMode ? "Failed to update config" : "Failed to create config", error)
        }
    }

    const showEditor = viewMode === 'editor' || viewMode === 'split';
    const showGraph = viewMode === 'graph' || viewMode === 'split';

    const containerHeight = height ?? (readOnly ? 400 : "calc(100vh - 92px)");

    return (
        <Box
            style={{
                display: "flex",
                flexDirection: "column",
                height: containerHeight,
                gap: 16,
            }}
        >
            {!readOnly && (
                <form onSubmit={form.onSubmit(handleSubmit)}>
                    <Group align="flex-end" gap="md">
                        <TextInput
                            withAsterisk
                            label="Config name"
                            placeholder="config"
                            disabled={isEditMode}
                            {...form.getInputProps('configName')}
                        />
                        <SegmentedControl
                            value={viewMode}
                            onChange={(value) => setViewMode(value as ViewMode)}
                            data={[
                                { label: 'Editor', value: 'editor' },
                                { label: 'Split', value: 'split' },
                                { label: 'Graph', value: 'graph' },
                            ]}
                        />
                        <Button type="submit">
                            {isEditMode ? 'Update config' : 'Save config'}
                        </Button>
                    </Group>
                </form>
            )}

            {readOnly && (
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
            )}

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
                            defaultValue={actualConfig}
                            value={configData}
                            width="100%"
                            height="100%"
                            defaultLanguage="yaml"
                            theme={monacoTheme}
                            options={{
                                readOnly: readOnly,
                                quickSuggestions: readOnly ? false : { other: true, strings: true },
                                automaticLayout: true,
                                minimap: { enabled: false },
                                scrollbar: { verticalScrollbarSize: 8, horizontal: "hidden" },
                                padding: { top: 5 },
                                fontSize: 13,
                                fontWeight: "400",
                            }}
                            onMount={handleEditorMount}
                            onChange={handleEditorChange}
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
                        <PipelineGraph key={viewMode} value={configData} />
                    </Paper>
                )}
            </Box>
        </Box>
    );
}
