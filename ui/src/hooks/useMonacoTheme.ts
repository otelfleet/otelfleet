import { useEffect, useRef } from 'react';
import { useComputedColorScheme } from '@mantine/core';
import { loader } from '@monaco-editor/react';
import {
  MONACO_LIGHT_THEME,
  MONACO_DARK_THEME,
  defineMonacoThemes,
} from '../theme/monaco';

/**
 * Hook that syncs Monaco editor theme with Mantine's color scheme.
 *
 * Registers custom Monaco themes on first use and returns the appropriate
 * theme name based on the current Mantine color scheme.
 *
 * @returns The Monaco theme name to use based on current color scheme
 *
 * @example
 * ```tsx
 * const theme = useMonacoTheme();
 * return <MonacoEditor theme={theme} ... />;
 * ```
 */
export function useMonacoTheme(): string {
  const computedColorScheme = useComputedColorScheme('light');
  const themesRegistered = useRef(false);

  useEffect(() => {
    if (themesRegistered.current) return;

    loader.init().then((monaco) => {
      defineMonacoThemes(monaco);
      themesRegistered.current = true;
    });
  }, []);

  return computedColorScheme === 'dark' ? MONACO_DARK_THEME : MONACO_LIGHT_THEME;
}
