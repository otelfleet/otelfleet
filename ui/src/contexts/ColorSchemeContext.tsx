import { createContext } from 'react';
import type { MantineColorScheme } from '@mantine/core';

export interface ColorSchemeContextValue {
  colorScheme: MantineColorScheme;
  setColorScheme: (scheme: MantineColorScheme) => void;
  toggleColorScheme: () => void;
}

const ColorSchemeContext = createContext<ColorSchemeContextValue | null>(null);

export default ColorSchemeContext;
