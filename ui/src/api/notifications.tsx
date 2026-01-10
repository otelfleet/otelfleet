import { notifications } from '@mantine/notifications';
import { ConnectError } from "@connectrpc/connect";
import { Code } from "@connectrpc/connect";
import { CrossCircledIcon } from '@radix-ui/react-icons';

export function notifyGRPCError(
    title : String,
    error: unknown,
) {
    const connectErr = ConnectError.from(error);
    notifications.show({
        title: title + " [" + Code[connectErr.code] + "]",
        message: connectErr.message,
        icon: <CrossCircledIcon />,
    })
}