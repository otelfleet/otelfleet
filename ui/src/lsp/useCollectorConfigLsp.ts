import { useEffect, useRef, useState } from 'react';
import type * as monacoNs from 'monaco-editor';
import { attachLsp } from './monacoLsp';

export type LspStatus = 'connecting' | 'connected' | 'disconnected' | 'error';

const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 15000;

function lspUrl(): string {
  const base = import.meta.env.VITE_API_BASE_URL || window.location.origin;
  const url = new URL('/lsp', base);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.toString();
}

interface LspSession {
  monaco: typeof monacoNs;
  editor: monacoNs.editor.IStandaloneCodeEditor;
}

// Attaches the CollectorConfig editor to the LSP server over WebSocket, syncing
// the document and rendering diagnostics as markers. Returns the live WebSocket
// connection status and reconnects with backoff.
export function useCollectorConfigLsp(
  session: LspSession | null,
  documentUri: string,
  enabled: boolean,
): LspStatus {
  const [status, setStatus] = useState<LspStatus>('connecting');
  const statusRef = useRef(setStatus);
  statusRef.current = setStatus;

  useEffect(() => {
    if (!session || !enabled) return;

    let disposed = false;
    let detach: (() => void) | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;

    const scheduleReconnect = () => {
      if (disposed) return;
      const delay = Math.min(RECONNECT_BASE_MS * 2 ** attempt, RECONNECT_MAX_MS);
      attempt += 1;
      reconnectTimer = setTimeout(connect, delay);
    };

    const connect = () => {
      if (disposed) return;
      statusRef.current('connecting');
      attachLsp(session.monaco, session.editor, lspUrl(), documentUri, {
        onClose: () => {
          if (disposed) return;
          statusRef.current('disconnected');
          scheduleReconnect();
        },
        onError: () => {
          if (!disposed) statusRef.current('error');
        },
      })
        .then((cleanup) => {
          if (disposed) {
            cleanup();
            return;
          }
          detach = cleanup;
          attempt = 0;
          statusRef.current('connected');
        })
        .catch(() => {
          if (disposed) return;
          statusRef.current('error');
          scheduleReconnect();
        });
    };

    connect();

    return () => {
      disposed = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      detach?.();
    };
  }, [session, documentUri, enabled]);

  return status;
}
