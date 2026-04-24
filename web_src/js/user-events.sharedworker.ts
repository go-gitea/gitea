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

  constructor(wsUrl: string, source: Source) {
    this.wsUrl = wsUrl;
    this.source = source;
    this.ws = null;
    this.reconnectTimer = null;
    this.reconnectDelay = 50;
    this.connect();
  }

  connect() {
    this.ws = new WebSocket(this.wsUrl);

    this.ws.addEventListener('open', () => {
      this.reconnectDelay = 50;
    });

    this.ws.addEventListener('message', (event: MessageEvent<string>) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'notification-count') {
          this.source.notifyClients({
            type: 'notification-count',
            data: JSON.stringify({Count: msg.count}),
          });
        } else if (msg.type === 'stopwatches') {
          this.source.notifyClients({
            type: 'stopwatches',
            data: JSON.stringify(msg.data),
          });
        } else if (msg.type === 'logout') {
          this.source.notifyClients({
            type: 'logout',
            data: msg.data ?? '',
          });
        }
      } catch {
        // ignore malformed messages
      }
    });

    this.ws.addEventListener('close', () => {
      this.ws = null;
      this.scheduleReconnect();
    });

    this.ws.addEventListener('error', () => {
      this.ws = null;
      this.scheduleReconnect();
    });
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

const sourcesByUrl = new Map<string, Source | null>();
const sourcesByPort = new Map<MessagePort, Source | null>();
const wsSourcesByUrl = new Map<string, WsSource | null>();

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
            sourcesByUrl.set(source.url, null);
            const ws = wsSourcesByUrl.get(source.url);
            if (ws) {
              ws.close();
              wsSourcesByUrl.set(source.url, null);
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
        if (count === 0) {
          sourcesByUrl.set(source.url, null);
          sourcesByPort.set(port, null);
          const ws = wsSourcesByUrl.get(source.url);
          if (ws) {
            ws.close();
            wsSourcesByUrl.set(source.url, null);
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
