import React, { useEffect, useRef, useState, useCallback } from "react";

type RenderProps = { width: number; height: number };

export function AutoSizer({
    children,
    className,
    style,
    headerSelector = ".mantine-AppShell-header, .mantine-AppShellHeader-root, header",
}: {
    children: (size: RenderProps) => React.ReactNode;
    className?: string;
    style?: React.CSSProperties;
    headerSelector?: string;
}) {
    const ref = useRef<HTMLDivElement | null>(null);
    const [size, setSize] = useState<RenderProps>({ width: 0, height: 0 });

    const measure = useCallback(() => {
        const el = ref.current;
        if (!el) return;
        const rect = el.getBoundingClientRect();
        const width = rect.width || (el.parentElement?.clientWidth ?? window.innerWidth);

        // default available height from element top to viewport bottom
        const viewportHeight = window.innerHeight;

        // find header element (if present) and subtract its height when it's above the element
        let headerHeight = 0;
        try {
            const headerEl = document.querySelector(headerSelector);
            if (headerEl instanceof Element) {
                const hRect = headerEl.getBoundingClientRect();
                if (hRect.top <= rect.top + 1) { // header is above (or overlapping) the element
                    headerHeight = hRect.height;
                }
            }
        } catch {
            headerHeight = 0;
        }

        const availableHeight = Math.max(0, viewportHeight - rect.top - headerHeight);
        setSize({ width: Math.floor(width), height: Math.floor(availableHeight) });
    }, [headerSelector]);

    useEffect(() => {
        measure();

        let ro: ResizeObserver | undefined;
        if (typeof ResizeObserver !== "undefined" && ref.current) {
            ro = new ResizeObserver(measure);
            ro.observe(ref.current);
            if (ref.current.parentElement) ro.observe(ref.current.parentElement);
        }

        window.addEventListener("resize", measure);
        window.addEventListener("orientationchange", measure);
        window.addEventListener("scroll", measure, true);

        const interval = setInterval(measure, 500);

        return () => {
            if (ro) ro.disconnect();
            window.removeEventListener("resize", measure);
            window.removeEventListener("orientationchange", measure);
            window.removeEventListener("scroll", measure, true);
            clearInterval(interval);
        };
    }, [measure]);

    return (
        <div ref={ref} className={className} style={{ width: "100%", ...style }}>
            {children(size)}
        </div>
    );
}