class Source {
  url: string;
  eventSource: EventSource | null;
  listening: Record<string, boolean>;
  clients: Array<MessagePort>;

  constructor(url: string) {
    this.url = url;
    this.eventSource = new EventSource(url);
    this.listening = {};
    this.clients = [];
    this.listen('open');
    this.listen('close');
    this.listen('logout');
    this.listen('notification-count');
    this.listen('stopwatches');
    this.listen('error');
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

  close() {
    if (!this.eventSource) return;

    this.eventSource.close();
    this.eventSource = null;
  }

  listen(eventType: string) {
    if (this.listening[eventType]) return;
    this.listening[eventType] = true;
    this.eventSource?.addEventListener(eventType, (event) => {
      this.notifyClients({
        type: eventType,
        data: event.data,
      });
    });
  }

  notifyClients(event: {type: string, data: any}) {
    for (const client of this.clients) {
      client.postMessage(event);
    }
  }

  status(port: MessagePort) {
    port.postMessage({
      type: 'status',
      message: `url: ${this.url} readyState: ${this.eventSource?.readyState}`,
    });
  }
}

// WsSource provides a WebSocket transport alongside EventSource.
// It delivers real-time notification-count pushes using the same client list
// as the associated Source, normalising messages to the SSE event format so
// that callers do not need to know which transport delivered the event.
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
          // Normalise to SSE event format so the receiver is transport-agnostic.
          this.source.notifyClients({
            type: 'notification-count',
            data: JSON.stringify({Count: msg.count}),
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
      if (!self.EventSource) {
        // some browsers (like PaleMoon, Firefox<53) don't support EventSource in SharedWorkerGlobalScope.
        // this event handler needs EventSource when doing "new Source(url)", so just post a message back to the caller,
        // in case the caller would like to use a fallback method to do its work.
        port.postMessage({type: 'no-event-source'});
        return;
      }
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
          if (source.eventSource && source.url === url) return;

          // How this has happened I don't understand...
          // deregister from that source
          const count = source.deregister(port);
          // Clean-up
          if (count === 0) {
            source.close();
            sourcesByUrl.set(source.url, null);
            const ws = wsSourcesByUrl.get(source.url);
            if (ws) {
              ws.close();
              wsSourcesByUrl.set(source.url, null);
            }
          }
        }
        // Create a new Source
        source = new Source(url);
        source.register(port);
        sourcesByUrl.set(url, source);
        sourcesByPort.set(port, source);
        // Start WebSocket alongside EventSource for real-time notification pushes.
        const wsUrl = url.replace(/^http/, 'ws').replace(/\/user\/events$/, '/-/ws');
        wsSourcesByUrl.set(url, new WsSource(wsUrl, source));
      } else if (event.data.type === 'listen') {
        const source = sourcesByPort.get(port)!;
        source.listen(event.data.eventType);
      } else if (event.data.type === 'close') {
        const source = sourcesByPort.get(port);
        if (!source) return;

        const count = source.deregister(port);
        if (count === 0) {
          source.close();
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
