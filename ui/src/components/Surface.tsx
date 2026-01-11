import { forwardRef } from 'react';
import { Box, type BoxProps } from '@mantine/core';
import { elevationShadows, type ElevationLevel } from '../theme/elevation';

export interface SurfaceProps extends BoxProps {
  /**
   * Elevation level determines the visual depth of the surface
   * - base: Deepest background (AppShell.Main) - no shadow, darkest in dark mode
   * - surface: Standard surfaces (cards, panels) - subtle shadow
   * - raised: Elevated elements (popovers, dropdowns) - medium shadow
   * - overlay: High-priority elements (modals) - strong shadow
   */
  elevation?: ElevationLevel;
  /**
   * Border radius - defaults to 'md'
   */
  radius?: BoxProps['style'] extends { borderRadius?: infer R } ? R : string | number;
}

/**
 * Surface component that applies the elevation system
 *
 * Use this component to wrap content that needs depth/elevation styling.
 * The elevation level determines both the background shade and shadow intensity.
 *
 * @example
 * ```tsx
 * <Surface elevation="surface">
 *   <Text>Card content</Text>
 * </Surface>
 *
 * <Surface elevation="raised">
 *   <Text>Dropdown content</Text>
 * </Surface>
 * ```
 */
export const Surface = forwardRef<HTMLDivElement, SurfaceProps>(
  ({ elevation = 'surface', radius = 'md', style, children, ...props }, ref) => {
    return (
      <Box
        ref={ref}
        style={{
          backgroundColor: `var(--elevation-${elevation}-bg)`,
          boxShadow: elevationShadows[elevation],
          borderRadius: typeof radius === 'number' ? `${radius}px` : `var(--mantine-radius-${radius})`,
          ...style,
        }}
        {...props}
      >
        {children}
      </Box>
    );
  }
);

Surface.displayName = 'Surface';

export default Surface;
