import React, { Suspense, useMemo, useEffect, useRef, useState } from 'react';
import {Editor as EditorBox} from '../components/Editor';
import {useClient} from "../api"
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';

// ...existing code...

function wrapPromise<T>(promise: Promise<T>) {
    let status: 'pending' | 'success' | 'error' = 'pending';
    let result: T | any;
    const suspender = promise.then(
        (r) => { status = 'success'; result = r; },
        (e) => { status = 'error'; result = e; }
    );
    return {
        read(): T {
            if (status === 'pending') throw suspender;
            if (status === 'error') throw result;
            return result;
        }
    };
}

export function Editor() {
    const client = useClient(ConfigService);

    const resource = useMemo(() => {
        const p = client.getDefaultConfig({}).then((response) => {
            const bytes = (response?.config ?? new Uint8Array()) as Uint8Array;
            return new TextDecoder().decode(bytes);
        });
        return wrapPromise<string | null>(p);
    }, [client]);

    function Loader() {
        
        const defaultConfig = resource.read();
        return (
                <EditorBox
                    defaultConfig={defaultConfig}
                />

        );
    }

    return (
        <Suspense fallback={null}>
            <Loader />
        </Suspense>
    );
}