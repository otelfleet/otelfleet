// Wires a Monaco editor model to the otelcol-lsp server: syncs the document and
// renders published diagnostics as Monaco markers.

import type * as monacoNs from 'monaco-editor';
import { LspClient } from './lspClient';

interface LspRange {
  start: { line: number; character: number };
  end: { line: number; character: number };
}

interface LspDiagnostic {
  range: LspRange;
  severity?: number;
  code?: string | number;
  source?: string;
  message: string;
}

interface PublishDiagnosticsParams {
  uri: string;
  diagnostics: Array<LspDiagnostic>;
}

const severityMap: Record<number, monacoNs.MarkerSeverity> = {
  1: 8, // Error
  2: 4, // Warning
  3: 2, // Information
  4: 1, // Hint
};

interface AttachLspEvents {
  onClose?: () => void;
  onError?: () => void;
}

export async function attachLsp(
  monaco: typeof monacoNs,
  editor: monacoNs.editor.IStandaloneCodeEditor,
  url: string,
  documentUri: string,
  events: AttachLspEvents = {},
): Promise<() => void> {
  const model = editor.getModel();
  if (!model) throw new Error('editor has no model');
  const uri = documentUri;

  // The server re-encodes the URI query (go.lsp.dev percent-encodes the
  // delimiters), so match on the path portion rather than the full string.
  const docPath = uri.split('?')[0];

  const client = await LspClient.connect(url, events);
  client.onNotification('textDocument/publishDiagnostics', (params) => {
    const { uri: reportedUri, diagnostics } = params as PublishDiagnosticsParams;
    if (reportedUri.split('?')[0] !== docPath) return;
    monaco.editor.setModelMarkers(
      model,
      'otelcol-lsp',
      diagnostics.map((d) => ({
        startLineNumber: d.range.start.line + 1,
        startColumn: d.range.start.character + 1,
        endLineNumber: d.range.end.line + 1,
        endColumn: d.range.end.character + 1,
        severity: severityMap[d.severity ?? 1],
        code: d.code?.toString(),
        source: d.source,
        message: d.message,
      })),
    );
  });

  await client.request('initialize', {
    processId: null,
    rootUri: null,
    capabilities: {},
  });
  client.notify('initialized', {});
  client.notify('textDocument/didOpen', {
    textDocument: {
      uri,
      languageId: model.getLanguageId(),
      version: model.getVersionId(),
      text: model.getValue(),
    },
  });

  const changeListener = model.onDidChangeContent(() => {
    client.notify('textDocument/didChange', {
      textDocument: { uri, version: model.getVersionId() },
      contentChanges: [{ text: model.getValue() }],
    });
  });

  return () => {
    changeListener.dispose();
    monaco.editor.setModelMarkers(model, 'otelcol-lsp', []);
    client.notify('textDocument/didClose', { textDocument: { uri } });
    client.close();
  };
}
