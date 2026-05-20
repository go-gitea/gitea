import {serverEventTypes} from './types.ts';
import type {ServerEventMessage, UserEventMessage} from './types.ts';

function isServerEventMessage(msg: unknown): msg is ServerEventMessage {
  if (!msg || typeof msg !== 'object' || !('type' in msg)) return false;
  return (serverEventTypes as ReadonlyArray<string>).includes(msg.type as string);
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
    });

    this.ws.addEventListener('message', (event: MessageEvent<string>) => {
      try {
        const msg: unknown = JSON.parse(event.data);
        if (isServerEventMessage(msg)) this.source.notifyClients(msg);
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

    this.ws.addEventListener('close', (event: CloseEvent) => {
      this.ws = null;
      if (this.closed) return;
      // Server signals an expired/missing session via the IANA "Unauthorized" close code; reconnecting can't recover that.
      if (event.code === 3000) {
        this.closed = true;
        return;
      }
      this.failuresWithoutConnect++;
      this.maybeSignalFallback();
      this.scheduleReconnect();
    });
  }

  maybeSignalFallback() {
    if (this.fallbackSignalled) return;
    if (this.failuresWithoutConnect < 3) return;
    this.fallbackSignalled = true;
    this.source.notifyClients({type: 'push-unavailable'});
  }

  scheduleReconnect() {
    if (this.reconnectTimer !== null) return;
    // Jitter 50%–150% of base delay to prevent thundering-herd reconnects after a server restart.
    const delay = this.reconnectDelay * (0.5 + Math.random());
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, 60000);
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
