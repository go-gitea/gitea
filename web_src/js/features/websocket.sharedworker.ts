// One WebSocket connection per URL, shared across all tabs via SharedWorker.
// Messages from the server are JSON objects broadcast to all connected ports.
export {}; // make this a module to avoid global scope conflicts with other sharedworker files

const RECONNECT_DELAY_INITIAL = 50;
const RECONNECT_DELAY_MAX = 10000;

class WsSource {
  url: string;
  ws: WebSocket | null;
  clients: MessagePort[];
  reconnectTimer: ReturnType<typeof setTimeout> | null;
  reconnectDelay: number;

  constructor(url: string) {
    this.url = url;
    this.ws = null;
    this.clients = [];
    this.reconnectTimer = null;
    this.reconnectDelay = RECONNECT_DELAY_INITIAL;
    this.connect();
  }

  connect() {
    this.ws = new WebSocket(this.url);

    this.ws.addEventListener('open', () => {
      this.reconnectDelay = RECONNECT_DELAY_INITIAL;
      this.broadcast({type: 'status', message: `connected to ${this.url}`});
    });

    this.ws.addEventListener('message', (event: MessageEvent<string>) => {
      try {
        const msg = JSON.parse(event.data);
        this.broadcast(msg);
      } catch {
        // ignore malformed JSON
      }
    });

    this.ws.addEventListener('close', () => {
      this.ws = null;
      this.scheduleReconnect();
    });

    this.ws.addEventListener('error', () => {
      this.broadcast({type: 'error', message: 'websocket error'});
      this.ws = null;
      this.scheduleReconnect();
    });
  }

  scheduleReconnect() {
    if (this.clients.length === 0 || this.reconnectTimer !== null) return;
    const delay = this.reconnectDelay;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, RECONNECT_DELAY_MAX);
  }

  register(port: MessagePort) {
    if (this.clients.includes(port)) return;
    this.clients.push(port);
    port.postMessage({type: 'status', message: `registered to ${this.url}`});
  }

  deregister(port: MessagePort): number {
    const idx = this.clients.indexOf(port);
    if (idx >= 0) this.clients.splice(idx, 1);
    return this.clients.length;
  }

  close() {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
  }

  broadcast(msg: unknown) {
    for (const port of this.clients) {
      port.postMessage(msg);
    }
  }
}

const sourcesByUrl = new Map<string, WsSource>();
const sourcesByPort = new Map<MessagePort, WsSource>();

(self as unknown as SharedWorkerGlobalScope).addEventListener('connect', (e: MessageEvent) => {
  for (const port of e.ports) {
    port.addEventListener('message', (event: MessageEvent) => {
      if (event.data.type === 'start') {
        const {url} = event.data;
        let source = sourcesByUrl.get(url);
        if (source) {
          source.register(port);
          sourcesByPort.set(port, source);
          return;
        }
        source = sourcesByPort.get(port);
        if (source) {
          const count = source.deregister(port);
          if (count === 0) {
            source.close();
            sourcesByUrl.delete(source.url);
          }
        }
        source = new WsSource(url);
        source.register(port);
        sourcesByUrl.set(url, source);
        sourcesByPort.set(port, source);
      } else if (event.data.type === 'close') {
        const source = sourcesByPort.get(port);
        if (!source) return;
        const count = source.deregister(port);
        if (count === 0) {
          source.close();
          sourcesByUrl.delete(source.url);
          sourcesByPort.delete(port);
        }
      } else if (event.data.type === 'status') {
        const source = sourcesByPort.get(port);
        if (!source) {
          port.postMessage({type: 'status', message: 'not connected'});
          return;
        }
        port.postMessage({
          type: 'status',
          message: `url: ${source.url} readyState: ${source.ws?.readyState ?? 'null'}`,
        });
      } else {
        port.postMessage({
          type: 'error',
          message: `received but don't know how to handle: ${JSON.stringify(event.data)}`,
        });
      }
    });
    port.start();
  }
});
