import {
  serverUserEventTypes,
  type SharedWorkerControlMessage,
  type UserEventMessage,
  type WorkerEventMessage,
  type WorkerInboundMessage,
} from './types.ts';

// chrome://inspect/#workers
let showDebugLog = false;

function logDebug(...args: any[]) {
  if (!showDebugLog) return;
  console.debug('[user-events.sharedworker]', ...args);
}

function isServerEventMessage(msg: unknown): msg is UserEventMessage {
  if (!msg || typeof msg !== 'object') return false;
  const userEvent = msg as UserEventMessage;
  return (serverUserEventTypes as ReadonlyArray<string>).includes(userEvent.eventType ?? '');
}

function postUserEventMessage(client: MessagePort, msgData: UserEventMessage) {
  logDebug('postUserEventMessage', msgData);
  const msg: WorkerInboundMessage = {msgType: 'user-event', msgData};
  client.postMessage(msg);
}

function postWorkerEventMessage(client: MessagePort, msgData: WorkerEventMessage) {
  logDebug('postWorkerEventMessage', msgData);
  const msg: WorkerInboundMessage = {msgType: 'worker-event', msgData};
  client.postMessage(msg);
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

  notifyClientsUserEvent(event: UserEventMessage) {
    for (const client of this.clients) {
      postUserEventMessage(client, event);
    }
  }
  notifyClientsWorkerEvent(event: WorkerEventMessage) {
    for (const client of this.clients) {
      postWorkerEventMessage(client, event);
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
      // Pushes fired while no client was subscribed (initial connect gap, or a
      // reconnect window) are dropped server-side, so tell clients to reconcile
      // their state from the server on every fresh connection.
      this.source.notifyClientsUserEvent({eventType: 'worker-connected'});
    });

    this.ws.addEventListener('message', (event: MessageEvent<string>) => {
      try {
        const msg: unknown = JSON.parse(event.data);
        logDebug('websocket message', event.data);
        if (!isServerEventMessage(msg)) {
          console.error('websocket message is not a valid server user event', msg);
          return;
        }
        this.source.notifyClientsUserEvent(msg);
      } catch (err) {
        console.error('user-events: dropping malformed WebSocket message', err);
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
    this.source.notifyClientsUserEvent({eventType: 'worker-unavailable'});
  }

  scheduleReconnect() {
    if (this.reconnectTimer !== null) return;
    // Jitter 50%–150% of base delay to prevent thundering-herd reconnects after a server restart.
    const delay = this.reconnectDelay * (0.5 + Math.random());
    logDebug(`scheduling reconnect in ${delay}ms`);
    this.reconnectTimer = setTimeout(() => {
      logDebug(`reconnecting ...`);
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
    port.addEventListener('message', (event: MessageEvent<SharedWorkerControlMessage>) => {
      if (event.data.type === 'start') {
        logDebug('received control message start', event.data);
        showDebugLog = event.data.showDebugLog;
        const url = event.data.url;
        let source = sourcesByUrl.get(url);
        if (source) {
          // we have a Source registered to this url
          source.register(port);
          sourcesByPort.set(port, source);
          // A port attaching to an already-open socket won't observe the next
          // "open", so replay "worker-connected" to it directly; otherwise a late tab
          // never learns the stream is live (mirrors the SSE sharedworker replaying
          // its built-in "open" event to late-attaching ports).
          const openWs = wsSourcesByUrl.get(url);
          if (openWs?.ws?.readyState === WebSocket.OPEN) {
            postUserEventMessage(port, {eventType: 'worker-connected'});
          }
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
        logDebug('received control message close', event.data);
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
        console.error('received control message unknown', event.data);
        postWorkerEventMessage(port, {workerEvent: 'error', message: `received but don't know how to handle: ${JSON.stringify(event.data)}`});
      }
    });
    port.start();
  }
});
