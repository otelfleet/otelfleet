import { Group, Text, Tooltip } from '@mantine/core';
import type { LspStatus } from './useCollectorConfigLsp';

interface LspStatusBadgeProps {
  status: LspStatus;
}

const PRESENTATION: Record<LspStatus, { color: string; label: string; tooltip: string; pulse: boolean }> = {
  connecting: { color: 'var(--mantine-color-yellow-6)', label: 'Connecting', tooltip: 'Connecting to the language server…', pulse: true },
  connected: { color: 'var(--mantine-color-green-6)', label: 'Connected', tooltip: 'Language server connected', pulse: false },
  disconnected: { color: 'var(--mantine-color-gray-5)', label: 'Offline', tooltip: 'Language server disconnected — retrying', pulse: false },
  error: { color: 'var(--mantine-color-red-6)', label: 'Unavailable', tooltip: 'Failed to reach the language server', pulse: false },
};

export function LspStatusBadge({ status }: LspStatusBadgeProps) {
  const { color, label, tooltip, pulse } = PRESENTATION[status];
  return (
    <Tooltip label={tooltip} withArrow>
      <Group gap={6} wrap="nowrap" align="center" mih="var(--input-height, 36px)" style={{ cursor: 'default' }}>
        <span
          style={{
            width: 8,
            height: 8,
            borderRadius: '50%',
            backgroundColor: color,
            animation: pulse ? 'lsp-pulse 1.2s ease-in-out infinite' : undefined,
          }}
        />
        <Text size="xs" c="dimmed">LSP · {label}</Text>
        <style>{'@keyframes lsp-pulse{0%,100%{opacity:1}50%{opacity:.35}}'}</style>
      </Group>
    </Tooltip>
  );
}
