// Minimal JSON-RPC 2.0 client over WebSocket, one message per frame.

type NotificationHandler = (params: unknown) => void;

interface Pending {
  resolve: (result: unknown) => void;
  reject: (error: Error) => void;
}

interface LspClientEvents {
  onClose?: () => void;
  onError?: () => void;
}

export class LspClient {
  private socket: WebSocket;
  private nextId = 1;
  private pending = new Map<number, Pending>();
  private handlers = new Map<string, NotificationHandler>();

  private constructor(socket: WebSocket, events: LspClientEvents) {
    this.socket = socket;
    socket.onmessage = (event) => this.dispatch(JSON.parse(event.data));
    socket.onclose = () => events.onClose?.();
    socket.onerror = () => events.onError?.();
  }

  static connect(url: string, events: LspClientEvents = {}): Promise<LspClient> {
    return new Promise((resolve, reject) => {
      const socket = new WebSocket(url);
      socket.onopen = () => resolve(new LspClient(socket, events));
      socket.onerror = () => reject(new Error(`failed to connect to ${url}`));
    });
  }

  request(method: string, params?: unknown): Promise<unknown> {
    const id = this.nextId++;
    this.socket.send(JSON.stringify({ jsonrpc: '2.0', id, method, params }));
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
    });
  }

  notify(method: string, params?: unknown): void {
    this.socket.send(JSON.stringify({ jsonrpc: '2.0', method, params }));
  }

  onNotification(method: string, handler: NotificationHandler): void {
    this.handlers.set(method, handler);
  }

  close(): void {
    this.socket.onclose = null;
    this.socket.close();
  }

  private dispatch(message: {
    id?: number;
    method?: string;
    params?: unknown;
    result?: unknown;
    error?: { message: string };
  }): void {
    if (message.method !== undefined) {
      this.handlers.get(message.method)?.(message.params);
      return;
    }
    if (message.id === undefined) return;
    const pending = this.pending.get(message.id);
    if (!pending) return;
    this.pending.delete(message.id);
    if (message.error) {
      pending.reject(new Error(message.error.message));
    } else {
      pending.resolve(message.result);
    }
  }
}
