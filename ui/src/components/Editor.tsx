
// SPDX-FileCopyrightText: 2023 Dash0 Inc.
// SPDX-License-Identifier: Apache-2.0


import { useState } from "react";
import MonacoEditor, { type OnChange, type OnMount } from "@monaco-editor/react";
import { Box, Button, Group, TextInput } from '@mantine/core';
import { useForm } from '@mantine/form'
import { useClient } from "../api";
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import { notifications } from "@mantine/notifications";
import { notifyGRPCError } from "../api/notifications";
import { useNavigate } from '@tanstack/react-router';
import { useMonacoTheme } from "../hooks/useMonacoTheme";


// ...existing code...

export function Editor(
    props: { defaultConfig?: string | null, containerWidth?: number, containerHeight?: number, style?: React.CSSProperties }) {
    const { defaultConfig } = props;

    const monacoTheme = useMonacoTheme();
    const actualConfig = defaultConfig ? defaultConfig : ""
    const [configData, setConfig] = useState(actualConfig)
    const handleEditorChange: OnChange = (value) => {
        if (value !== undefined) {
            console.log(value)
            setConfig(value);
        }
    }

    const handleEditorMount: OnMount = (editor) => {
        // Force layout recalculation to fix dimension issues
        requestAnimationFrame(() => {
            editor.layout();
        });
    }

    const form = useForm({
        mode: 'uncontrolled',
        initialValues: {
            configName: '',
        },

        validate: {
            configName: (value) => (/[a-zA-Z0-9]/.test(value) ? null : 'Invalid config name'),
        },
    });

    const otelConfigClient = useClient(ConfigService)
    const navigate = useNavigate();

    return (
        <Box
            style={{
                display: "flex",
                flexDirection: "column",
                height: "calc(100vh - 92px)",
                gap: 16,
            }}
        >
            <form
                onSubmit={form.onSubmit((values) => {
                    console.log("putting config")
                    try {
                        const bytes = new TextEncoder().encode(configData);
                        otelConfigClient.putConfig({
                            ref: {
                                id: values.configName,
                            },
                            config: {
                                config: bytes,
                            },
                        })
                        notifications.show({
                            title: "Created OpenTelemetry config",
                            message: "created config : `" + values.configName + "`",
                        })
                        navigate({
                            to: '/configs',
                        });
                    } catch (error) {
                        notifyGRPCError("Failed to create config", error)
                    }
                })}
            >
                <Group align="flex-end" gap="md">
                    <TextInput
                        withAsterisk
                        label="Config name"
                        placeholder="config"
                        key={form.key('configName')}
                        {...form.getInputProps('configName')}
                    />
                    <Button type="submit">Save config</Button>
                </Group>
            </form>
            <Box style={{ flex: 1, minHeight: 0 }}>
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
            </Box>
        </Box>
    );
}
