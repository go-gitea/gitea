import {
  USER_EVENT_LOGOUT,
  USER_EVENT_NOTIFICATION_COUNT,
  USER_EVENT_PUSH_UNAVAILABLE,
  USER_EVENT_STOPWATCHES,
  USER_EVENT_WS_OPENED,
} from './modules/user-events-types.ts';

// translateServerMessage converts a server-sent WebSocket message into the
// {type, data} envelope consumed by page-side listeners. Returns null for
// unknown message types so they are dropped silently.
function translateServerMessage(msg: {type: string, count?: number, data?: any}): {type: string, data: any} | null {
  if (msg.type === USER_EVENT_NOTIFICATION_COUNT) {
    return {type: USER_EVENT_NOTIFICATION_COUNT, data: JSON.stringify({Count: msg.count})};
  }
  if (msg.type === USER_EVENT_STOPWATCHES) {
    return {type: USER_EVENT_STOPWATCHES, data: JSON.stringify(msg.data)};
  }
  if (msg.type === USER_EVENT_LOGOUT) {
    return {type: USER_EVENT_LOGOUT, data: msg.data ?? ''};
  }
  return null;
}

// Source manages the list of connected page ports for one logical connection.
// Real-time data is delivered by the accompanying WsSource over WebSocket.
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

  notifyClients(event: {type: string, data: any}) {
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

// WsSource provides a WebSocket transport for real-time event delivery.
// It dispatches messages through the Source so each page port receives
// a `{type, data}` event per message.
class WsSource {
  wsUrl: string;
  ws: WebSocket | null;
  source: Source;
  reconnectTimer: ReturnType<typeof setTimeout> | null;
  reconnectDelay: number;
  failuresWithoutConnect: number;
  fallbackSignalled: boolean;

  constructor(wsUrl: string, source: Source) {
    this.wsUrl = wsUrl;
    this.source = source;
    this.ws = null;
    this.reconnectTimer = null;
    this.reconnectDelay = 50;
    this.failuresWithoutConnect = 0;
    this.fallbackSignalled = false;
    this.connect();
  }

  connect() {
    this.ws = new WebSocket(this.wsUrl);

    this.ws.addEventListener('open', () => {
      this.reconnectDelay = 50;
      this.failuresWithoutConnect = 0;
      this.source.notifyClients({type: USER_EVENT_WS_OPENED, data: ''});
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
      this.failuresWithoutConnect++;
      this.maybeSignalFallback();
      this.scheduleReconnect();
    });
  }

  // If the WebSocket repeatedly fails to connect, tell page listeners once so
  // they can fall back to periodic polling instead of relying on pushes.
  maybeSignalFallback() {
    if (this.fallbackSignalled) return;
    if (this.failuresWithoutConnect < 3) return;
    this.fallbackSignalled = true;
    this.source.notifyClients({type: USER_EVENT_PUSH_UNAVAILABLE, data: ''});
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
