// Icons for OpenTelemetry pipeline components using Radix icons
import {
    DownloadIcon,
    UploadIcon,
    GearIcon,
    Link2Icon,
    ActivityLogIcon,
    BarChartIcon,
    ListBulletIcon,
} from '@radix-ui/react-icons';

interface IconProps {
    size?: number;
}

export function ReceiverIcon({ size = 20 }: IconProps) {
    return <DownloadIcon width={size} height={size} />;
}

export function ProcessorIcon({ size = 20 }: IconProps) {
    return <GearIcon width={size} height={size} />;
}

export function ExporterIcon({ size = 20 }: IconProps) {
    return <UploadIcon width={size} height={size} />;
}

export function ConnectorIcon({ size = 20 }: IconProps) {
    return <Link2Icon width={size} height={size} />;
}

// Pipeline type icons
export function TracesIcon({ size = 12 }: IconProps) {
    return <ActivityLogIcon width={size} height={size} />;
}

export function MetricsIcon({ size = 12 }: IconProps) {
    return <BarChartIcon width={size} height={size} />;
}

export function LogsIcon({ size = 12 }: IconProps) {
    return <ListBulletIcon width={size} height={size} />;
}
