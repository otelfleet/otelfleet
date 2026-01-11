import { useEffect, useRef, useState, useCallback, type ReactNode, type CSSProperties } from "react";

interface RenderProps {
    width: number;
    height: number;
}

interface AutoSizerProps {
    children: (size: RenderProps) => ReactNode;
    className?: string;
    style?: CSSProperties;
}

/**
 * A container component that measures its available space and passes
 * width/height to children via render props.
 *
 * The component fills its parent container and reports the measured dimensions.
 * Uses ResizeObserver for efficient resize detection.
 *
 * @example
 * ```tsx
 * <AutoSizer>
 *   {({ width, height }) => (
 *     <MonacoEditor width={width} height={height} />
 *   )}
 * </AutoSizer>
 * ```
 */
export function AutoSizer({ children, className, style }: AutoSizerProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const [size, setSize] = useState<RenderProps>({ width: 0, height: 0 });

    const measure = useCallback(() => {
        const el = containerRef.current;
        if (!el) return;

        // Use getBoundingClientRect for more reliable measurements
        const rect = el.getBoundingClientRect();
        const width = Math.floor(rect.width);
        const height = Math.floor(rect.height);

        if (width <= 0 || height <= 0) return;

        setSize((prev) => {
            if (prev.width === width && prev.height === height) {
                return prev;
            }
            return { width, height };
        });
    }, []);

    useEffect(() => {
        // Delay initial measurement to allow layout to compute
        const rafId = requestAnimationFrame(() => {
            measure();
        });

        const el = containerRef.current;
        if (!el) return;

        const resizeObserver = new ResizeObserver(measure);
        resizeObserver.observe(el);

        return () => {
            cancelAnimationFrame(rafId);
            resizeObserver.disconnect();
        };
    }, [measure]);

    return (
        <div
            ref={containerRef}
            className={className}
            style={{
                flex: 1,
                minWidth: 0,
                minHeight: 0,
                overflow: "hidden",
                ...style,
            }}
        >
            {size.width > 0 && size.height > 0 ? children(size) : null}
        </div>
    );
}
