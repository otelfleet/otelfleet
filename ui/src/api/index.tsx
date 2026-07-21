import { useMemo } from "react";
import { type DescService } from "@bufbuild/protobuf";
import { createConnectTransport } from "@connectrpc/connect-web";
import { createClient, type Client } from "@connectrpc/connect";

const baseUrl = import.meta.env.VITE_API_BASE_URL || window.location.origin;

// This transport is going to be used throughout the app
const transport = createConnectTransport({
  baseUrl,
});

/**
* Get a promise client for the given service.
*/
export function useClient<T extends DescService>(service: T): Client<T> {
  // We memoize the client, so that we only create one instance per service.
  return useMemo(() => createClient(service, transport), [service]);
}