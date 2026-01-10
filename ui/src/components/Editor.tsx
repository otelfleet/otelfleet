
// SPDX-FileCopyrightText: 2023 Dash0 Inc.
// SPDX-License-Identifier: Apache-2.0


import React, { useState, useRef, useEffect } from "react";
import { AutoSizer } from "./Autosizer";
import { useElementSize } from "@mantine/hooks";
import MonacoEditor, { loader, type OnChange } from "@monaco-editor/react";
import Flow from "../pipelines/Pipeline";
import { Button, Checkbox, Group, TextInput } from '@mantine/core';
import { useForm } from '@mantine/form'
import { useClient } from "../api";
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';
import { notifications } from "@mantine/notifications";
import { notifyGRPCError } from "../api/notifications";
import { useNavigate } from '@tanstack/react-router';


// ...existing code...

export function Editor(
    props: { defaultConfig?: string | null, containerWidth?: number, containerHeight?: number, style?: React.CSSProperties }) {
    const { defaultConfig, containerWidth: propWidth, containerHeight: propHeight, style: propStyle } = props;


    const actualConfig = defaultConfig ? defaultConfig : ""
    const [configData, setConfig] = useState(actualConfig)
    const handleEditorChange: OnChange = (value) => {
        if (value !== undefined) {
            console.log(value)
            setConfig(value);
        }
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
        <Group style={{ width: "100%", height: "100%", display: "flex", flex: 1, minHeight: 0 }} align="stretch">
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
                                // configData,
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
                style={{ display: 'flex', alignItems: 'center', gap: 12 }}
            >
                <TextInput
                    withAsterisk
                    label="Config name"
                    placeholder="config"
                    key={form.key('configName')}
                    {...form.getInputProps('configName')}
                />

                <Group justify="flex-end" mt="md">
                    <Button type="submit">Save config</Button>
                </Group>
            </form>
            <AutoSizer>
                {({ width, height }) => (
                    <MonacoEditor
                        defaultValue={actualConfig}
                        value={configData}
                        // onMount={editorDidMount}
                        width={width}
                        height={height}
                        defaultLanguage="yaml"
                        theme="OTelBin"
                        options={{
                            quickSuggestions: { other: true, strings: true },
                            automaticLayout: true,
                            minimap: { enabled: false },
                            scrollbar: { verticalScrollbarSize: 8, horizontal: "hidden" },
                            padding: { top: 5 },
                            fontSize: 13,
                            fontWeight: "400",
                            // fontFamily: firaCode.style.fontFamily,
                        }}
                        onChange={handleEditorChange}
                    />
                )}
            </AutoSizer>
        </Group>
    );
}
