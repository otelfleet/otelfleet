/**
 * Elevation System for UI Depth
 *
 * Based on the shade/shadow design paradigm:
 * - Elements closer to the user (foreground) appear lighter
 * - Elements further back (background) appear darker
 * - Shadows increase with elevation level
 *
 * Elevation Levels:
 * - base (0): The deepest background layer (AppShell.Main)
 * - surface (1): Standard surfaces like cards and panels
 * - raised (2): Elevated elements like popovers, dropdowns
 * - overlay (3): High-priority elements like modals, dialogs
 */

export type ElevationLevel = 'base' | 'surface' | 'raised' | 'overlay';

/**
 * Shadow definitions for each elevation level
 * Using layered shadows (ambient + key) for realistic depth
 * Shadows are more pronounced to create clear visual hierarchy
 */
export const elevationShadows = {
  base: 'none',
  surface: '0 2px 8px rgba(0, 0, 0, 0.12), 0 1px 3px rgba(0, 0, 0, 0.08)',
  raised: '0 4px 12px rgba(0, 0, 0, 0.15), 0 2px 4px rgba(0, 0, 0, 0.1)',
  overlay: '0 12px 28px rgba(0, 0, 0, 0.2), 0 4px 10px rgba(0, 0, 0, 0.12)',
} as const;

/**
 * Surface color indices in the Mantine dark color array
 * In dark mode, higher elevation = lighter color (simulating light reflection)
 * Index 0 is lightest, index 9 is darkest
 */
export const darkSurfaceShades = {
  base: 8,      // Darkest - deep background
  surface: 7,   // Card/panel level
  raised: 6,    // Popover/dropdown level
  overlay: 5,   // Modal/dialog level (lightest)
} as const;

/**
 * Surface color indices for light mode
 * In light mode, we use gray shades for subtle depth differentiation
 * while keeping the overall appearance light
 */
export const lightSurfaceShades = {
  base: 1,      // Very light gray background
  surface: 0,   // White for cards/panels (comes forward)
  raised: 0,    // White with shadow
  overlay: 0,   // White with stronger shadow
} as const;

/**
 * Get CSS variable references for elevation backgrounds
 * These work with Mantine's color scheme system
 */
export const getElevationBackground = (level: ElevationLevel): string => {
  return `var(--elevation-${level}-bg)`;
};

/**
 * CSS custom properties to inject into the theme
 * These adapt based on the current color scheme
 */
export const elevationCSSVariables = {
  light: {
    '--elevation-base-bg': 'var(--mantine-color-gray-1)',
    '--elevation-surface-bg': 'var(--mantine-color-white)',
    '--elevation-raised-bg': 'var(--mantine-color-white)',
    '--elevation-overlay-bg': 'var(--mantine-color-white)',
  },
  dark: {
    '--elevation-base-bg': 'var(--mantine-color-dark-8)',
    '--elevation-surface-bg': 'var(--mantine-color-dark-7)',
    '--elevation-raised-bg': 'var(--mantine-color-dark-6)',
    '--elevation-overlay-bg': 'var(--mantine-color-dark-5)',
  },
} as const;

/**
 * Mantine component style overrides for elevation system
 */
export const elevationStylesOverrides = {
  // AppShell uses base elevation for main, surface for header/navbar
  AppShell: {
    main: {
      backgroundColor: 'var(--elevation-base-bg)',
    },
    header: {
      backgroundColor: 'var(--elevation-surface-bg)',
      boxShadow: elevationShadows.surface,
      borderBottom: '1px solid var(--mantine-color-default-border)',
    },
    navbar: {
      backgroundColor: 'var(--elevation-surface-bg)',
      borderRight: '1px solid var(--mantine-color-default-border)',
    },
  },
  // Paper components are surfaces
  Paper: {
    root: {
      backgroundColor: 'var(--elevation-surface-bg)',
      border: '1px solid var(--mantine-color-default-border)',
    },
  },
  // Cards are surfaces
  Card: {
    root: {
      backgroundColor: 'var(--elevation-surface-bg)',
      boxShadow: elevationShadows.surface,
      border: '1px solid var(--mantine-color-default-border)',
    },
  },
  // Modals are overlay level
  Modal: {
    content: {
      backgroundColor: 'var(--elevation-overlay-bg)',
      boxShadow: elevationShadows.overlay,
    },
  },
  // Menus/Dropdowns are raised level
  Menu: {
    dropdown: {
      backgroundColor: 'var(--elevation-raised-bg)',
      boxShadow: elevationShadows.raised,
      border: '1px solid var(--mantine-color-default-border)',
      backdropFilter: 'none',
    },
  },
  // Popovers are raised level
  Popover: {
    dropdown: {
      backgroundColor: 'var(--elevation-raised-bg)',
      boxShadow: elevationShadows.raised,
      border: '1px solid var(--mantine-color-default-border)',
      backdropFilter: 'none',
    },
  },
} as const;
