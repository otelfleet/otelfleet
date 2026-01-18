import { memo } from 'react';

function EmptyStateNode() {
    return (
        <div
            style={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                padding: 24,
                backgroundColor: '#2b2c3d',
                borderRadius: 8,
                border: '1px dashed #4d4f66',
                color: '#9CA2AB',
                textAlign: 'center',
                minWidth: 300,
            }}
        >
            <svg
                width={48}
                height={48}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                style={{ marginBottom: 12, opacity: 0.5 }}
            >
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
            <div style={{ fontSize: 14, fontWeight: 500, marginBottom: 4 }}>
                No pipelines configured
            </div>
            <div style={{ fontSize: 12, opacity: 0.7 }}>
                Define service pipelines in your YAML config to see the visualization
            </div>
        </div>
    );
}

export default memo(EmptyStateNode);
