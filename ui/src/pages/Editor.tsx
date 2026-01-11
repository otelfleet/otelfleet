import { Suspense, useMemo } from 'react';
import { Editor as EditorBox } from '../components/Editor';
import { useClient } from "../api"
import { ConfigService } from '../gen/api/pkg/api/config/v1alpha1/config_pb';

function wrapPromise<T>(promise: Promise<T>) {
    let status: 'pending' | 'success' | 'error' = 'pending';
    let result: T;
    let error: Error;
    const suspender = promise.then(
        (r) => { status = 'success'; result = r; },
        (e) => { status = 'error'; error = e; }
    );
    return {
        read(): T {
            if (status === 'pending') throw suspender;
            if (status === 'error') throw error;
            return result;
        }
    };
}

interface EditorProps {
    configId?: string;
}

export function Editor({ configId }: EditorProps) {
    const client = useClient(ConfigService);

    const resource = useMemo(() => {
        const p = configId
            ? client.getConfig({ id: configId }).then((response) => {
                const bytes = (response?.config ?? new Uint8Array()) as Uint8Array;
                return new TextDecoder().decode(bytes);
            })
            : client.getDefaultConfig({}).then((response) => {
                const bytes = (response?.config ?? new Uint8Array()) as Uint8Array;
                return new TextDecoder().decode(bytes);
            });
        return wrapPromise<string | null>(p);
    }, [client, configId]);

    function Loader() {
        const defaultConfig = resource.read();
        return (
            <EditorBox
                defaultConfig={defaultConfig}
                configId={configId}
            />
        );
    }

    return (
        <Suspense fallback={null}>
            <Loader />
        </Suspense>
    );
}
