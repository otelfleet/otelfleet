import { useState } from "react";
import MonacoEditor, { type OnChange, type OnMount } from "@monaco-editor/react";
import { Box, Button, Group, TextInput, Paper } from '@mantine/core';
import { useForm } from '@mantine/form'
import { useClient } from "../api";
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import { notifications } from "@mantine/notifications";
import { notifyGRPCError } from "../api/notifications";
import { useNavigate } from '@tanstack/react-router';
import { useMonacoTheme } from "../hooks/useMonacoTheme";
import { CheckCircledIcon } from '@radix-ui/react-icons';

interface EditorProps {
    defaultConfig?: string | null;
    configId?: string;
    containerWidth?: number;
    containerHeight?: number;
    style?: React.CSSProperties;
}

export function Editor({ defaultConfig, configId }: EditorProps) {
    const isEditMode = Boolean(configId);

    const monacoTheme = useMonacoTheme();
    const actualConfig = defaultConfig ? defaultConfig : ""
    const [configData, setConfig] = useState(actualConfig)

    const handleEditorChange: OnChange = (value) => {
        if (value !== undefined) {
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

    return (
        <Box
            style={{
                display: "flex",
                flexDirection: "column",
                height: "calc(100vh - 92px)",
                gap: 16,
            }}
        >
            <form onSubmit={form.onSubmit(handleSubmit)}>
                <Group align="flex-end" gap="md">
                    <TextInput
                        withAsterisk
                        label="Config name"
                        placeholder="config"
                        disabled={isEditMode}
                        {...form.getInputProps('configName')}
                    />
                    <Button type="submit">
                        {isEditMode ? 'Update config' : 'Save config'}
                    </Button>
                </Group>
            </form>
            <Paper shadow="sm" radius="md" style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
                <MonacoEditor
                    defaultValue={actualConfig}
                    value={configData}
                    width="100%"
                    height="100%"
                    defaultLanguage="yaml"
                    theme={monacoTheme}
                    options={{
                        quickSuggestions: { other: true, strings: true },
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
        </Box>
    );
}
