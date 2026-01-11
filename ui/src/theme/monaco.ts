import type { editor } from 'monaco-editor';

export const MONACO_LIGHT_THEME = 'otelfleet-light';
export const MONACO_DARK_THEME = 'otelfleet-dark';

/**
 * Light theme for Monaco editor
 * Designed to complement Mantine's light mode
 */
export const lightTheme: editor.IStandaloneThemeData = {
  base: 'vs',
  inherit: true,
  rules: [
    { token: 'comment', foreground: '6a737d', fontStyle: 'italic' },
    { token: 'keyword', foreground: '5c68ac' },
    { token: 'string', foreground: '22863a' },
    { token: 'number', foreground: '005cc5' },
    { token: 'type', foreground: '6f42c1' },
    { token: 'key', foreground: '5474b4' },
    { token: 'delimiter', foreground: '24292e' },
    { token: 'tag', foreground: '22863a' },
    { token: 'attribute.name', foreground: '6f42c1' },
    { token: 'attribute.value', foreground: '032f62' },
  ],
  colors: {
    'editor.background': '#ffffff',
    'editor.foreground': '#24292e',
    'editor.lineHighlightBackground': '#f6f8fa',
    'editor.selectionBackground': '#c8d1e0',
    'editorCursor.foreground': '#5474b4',
    'editorLineNumber.foreground': '#6a737d',
    'editorLineNumber.activeForeground': '#24292e',
    'editorIndentGuide.background': '#e1e4e8',
    'editorIndentGuide.activeBackground': '#c8c8c8',
    'editorBracketMatch.background': '#c8d1e080',
    'editorBracketMatch.border': '#5474b4',
  },
};

/**
 * Dark theme for Monaco editor
 * Designed to complement Mantine's dark mode using the custom dark palette
 */
export const darkTheme: editor.IStandaloneThemeData = {
  base: 'vs-dark',
  inherit: true,
  rules: [
    { token: 'comment', foreground: '8c8fa3', fontStyle: 'italic' },
    { token: 'keyword', foreground: '7a84ba' },
    { token: 'string', foreground: '85e89d' },
    { token: 'number', foreground: '79b8ff' },
    { token: 'type', foreground: 'b392f0' },
    { token: 'key', foreground: '79b8ff' },
    { token: 'delimiter', foreground: 'd5d7e0' },
    { token: 'tag', foreground: '85e89d' },
    { token: 'attribute.name', foreground: 'b392f0' },
    { token: 'attribute.value', foreground: '9ecbff' },
  ],
  colors: {
    'editor.background': '#1d1e30',
    'editor.foreground': '#d5d7e0',
    'editor.lineHighlightBackground': '#2b2c3d',
    'editor.selectionBackground': '#4d4f6680',
    'editorCursor.foreground': '#7a84ba',
    'editorLineNumber.foreground': '#666980',
    'editorLineNumber.activeForeground': '#acaebf',
    'editorIndentGuide.background': '#34354a',
    'editorIndentGuide.activeBackground': '#4d4f66',
    'editorBracketMatch.background': '#4d4f6680',
    'editorBracketMatch.border': '#7a84ba',
  },
};

/**
 * Register Monaco themes with the Monaco editor instance
 * Should be called before any editor is mounted
 */
export function defineMonacoThemes(monaco: typeof import('monaco-editor')): void {
  monaco.editor.defineTheme(MONACO_LIGHT_THEME, lightTheme);
  monaco.editor.defineTheme(MONACO_DARK_THEME, darkTheme);
}
