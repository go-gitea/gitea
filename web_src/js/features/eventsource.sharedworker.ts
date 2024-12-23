class Source {
  url: string;
  eventSource: EventSource;
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
    this.eventSource.addEventListener(eventType, (event) => {
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
      message: `url: ${this.url} readyState: ${this.eventSource.readyState}`,
    });
  }
}

const sourcesByUrl: Map<string, Source | null> = new Map();
const sourcesByPort: Map<MessagePort, Source | null> = new Map();

// @ts-expect-error: typescript bug?
self.addEventListener('connect', (e: MessageEvent) => {
  for (const port of e.ports) {
    port.addEventListener('message', (event) => {
      if (!self.EventSource) {
        // some browsers (like PaleMoon, Firefox<53) don't support EventSource in SharedWorkerGlobalScope.
        // this event handler needs EventSource when doing "new Source(url)", so just post a message back to the caller,
        // in case the caller would like to use a fallback method to do its work.
        port.postMessage({type: 'no-event-source'});
        return;
      }
      if (event.data.type === 'start') {
        const url = event.data.url;
        if (sourcesByUrl.get(url)) {
          // we have a Source registered to this url
          const source = sourcesByUrl.get(url);
          source.register(port);
          sourcesByPort.set(port, source);
          return;
        }
        let source = sourcesByPort.get(port);
        if (source) {
          if (source.eventSource && source.url === url) return;

          // How this has happened I don't understand...
          // deregister from that source
          const count = source.deregister(port);
          // Clean-up
          if (count === 0) {
            source.close();
            sourcesByUrl.set(source.url, null);
          }
        }
        // Create a new Source
        source = new Source(url);
        source.register(port);
        sourcesByUrl.set(url, source);
        sourcesByPort.set(port, source);
      } else if (event.data.type === 'listen') {
        const source = sourcesByPort.get(port);
        source.listen(event.data.eventType);
      } else if (event.data.type === 'close') {
        const source = sourcesByPort.get(port);

        if (!source) return;

        const count = source.deregister(port);
        if (count === 0) {
          source.close();
          sourcesByUrl.set(source.url, null);
          sourcesByPort.set(port, null);
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
