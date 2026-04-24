import type {UserEventMessage} from './modules/user-events-types.ts';

// Returns null for unknown message types so they are dropped silently.
function translateServerMessage(msg: {type: string, count?: number, data?: any}): UserEventMessage | null {
  if (msg.type === 'notification-count') {
    return {type: 'notification-count', data: JSON.stringify({Count: msg.count})};
  }
  if (msg.type === 'stopwatches') {
    return {type: 'stopwatches', data: JSON.stringify(msg.data)};
  }
  if (msg.type === 'logout') {
    return {type: 'logout', data: msg.data ?? ''};
  }
  return null;
}

class Source {
  url: string;
  clients: Array<MessagePort>;

  constructor(url: string) {
    this.url = url;
    this.clients = [];
  }

  register(port: MessagePort) {
    if (this.clients.includes(port)) return;

    this.clients.push(port);

    port.postMessage({
      type: 'status',
      message: `registered to ${this.url}`,
    });
  }

  deregister(port: MessagePort) {
    const portIdx = this.clients.indexOf(port);
    if (portIdx < 0) {
      return this.clients.length;
    }
    this.clients.splice(portIdx, 1);
    return this.clients.length;
  }

  notifyClients(event: UserEventMessage) {
    for (const client of this.clients) {
      client.postMessage(event);
    }
  }

  status(port: MessagePort) {
    port.postMessage({
      type: 'status',
      message: `url: ${this.url}`,
    });
  }
}

class WsSource {
  wsUrl: string;
  ws: WebSocket | null;
  source: Source;
  reconnectTimer: ReturnType<typeof setTimeout> | null;
  reconnectDelay: number;
  failuresWithoutConnect: number;
  fallbackSignalled: boolean;
  closed: boolean;

  constructor(wsUrl: string, source: Source) {
    this.wsUrl = wsUrl;
    this.source = source;
    this.ws = null;
    this.reconnectTimer = null;
    this.reconnectDelay = 1000;
    this.failuresWithoutConnect = 0;
    this.fallbackSignalled = false;
    this.closed = false;
    this.connect();
  }

  connect() {
    if (this.closed) return;
    this.ws = new WebSocket(this.wsUrl);

    this.ws.addEventListener('open', () => {
      this.reconnectDelay = 1000;
      this.failuresWithoutConnect = 0;
      this.source.notifyClients({type: 'ws-opened', data: ''});
    });

    this.ws.addEventListener('message', (event: MessageEvent<string>) => {
      try {
        const msg = JSON.parse(event.data);
        const forwarded = translateServerMessage(msg);
        if (forwarded) this.source.notifyClients(forwarded);
      } catch (err) {
        console.warn('user-events: dropping malformed WebSocket message', err);
      }
    });

    // `error` always fires before `close` on a failed connection, so we count
    // failures and schedule reconnects from `close` only — otherwise the
    // fallback threshold would trip after two real failures instead of three.
    this.ws.addEventListener('error', () => {
      this.ws = null;
    });

    this.ws.addEventListener('close', () => {
      this.ws = null;
      if (this.closed) return;
      this.failuresWithoutConnect++;
      this.maybeSignalFallback();
      this.scheduleReconnect();
    });
  }

  maybeSignalFallback() {
    if (this.fallbackSignalled) return;
    if (this.failuresWithoutConnect < 3) return;
    this.fallbackSignalled = true;
    this.source.notifyClients({type: 'push-unavailable', data: ''});
  }

  scheduleReconnect() {
    if (this.reconnectTimer !== null) return;
    const delay = this.reconnectDelay;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, 10000);
  }

  close() {
    this.closed = true;
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
  }
}

const sourcesByUrl = new Map<string, Source>();
const sourcesByPort = new Map<MessagePort, Source>();
const wsSourcesByUrl = new Map<string, WsSource>();

(self as unknown as SharedWorkerGlobalScope).addEventListener('connect', (e: MessageEvent) => {
  for (const port of e.ports) {
    port.addEventListener('message', (event: MessageEvent) => {
      if (event.data.type === 'start') {
        const url = event.data.url;
        let source = sourcesByUrl.get(url);
        if (source) {
          // we have a Source registered to this url
          source.register(port);
          sourcesByPort.set(port, source);
          return;
        }
        source = sourcesByPort.get(port);
        if (source) {
          if (source.url === url) return;

          // How this has happened I don't understand...
          // deregister from that source
          const count = source.deregister(port);
          // Clean-up
          if (count === 0) {
            sourcesByUrl.delete(source.url);
            const ws = wsSourcesByUrl.get(source.url);
            if (ws) {
              ws.close();
              wsSourcesByUrl.delete(source.url);
            }
          }
        }
        // Create a new Source and its WebSocket transport
        source = new Source(url);
        source.register(port);
        sourcesByUrl.set(url, source);
        sourcesByPort.set(port, source);
        wsSourcesByUrl.set(url, new WsSource(url, source));
      } else if (event.data.type === 'close') {
        const source = sourcesByPort.get(port);
        if (!source) return;

        const count = source.deregister(port);
        sourcesByPort.delete(port);
        if (count === 0) {
          sourcesByUrl.delete(source.url);
          const ws = wsSourcesByUrl.get(source.url);
          if (ws) {
            ws.close();
            wsSourcesByUrl.delete(source.url);
          }
        }
      } else if (event.data.type === 'status') {
        const source = sourcesByPort.get(port);
        if (!source) {
          port.postMessage({
            type: 'status',
            message: 'not connected',
          });
          return;
        }
        source.status(port);
      } else {
        // just send it back
        port.postMessage({
          type: 'error',
          message: `received but don't know how to handle: ${event.data}`,
        });
      }
    });
    port.start();
  }
});
