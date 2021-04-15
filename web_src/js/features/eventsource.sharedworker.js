self.name = 'eventsource.sharedworker.js';

const sourcesByUrl = {};
const sourcesByPort = {};

class Source {
  constructor(url) {
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

  register(port) {
    if (this.clients.includes(port)) return;

    this.clients.push(port);

    port.postMessage({
      type: 'status',
      message: `registered to ${this.url}`,
    });
  }

  deregister(port) {
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

  listen(eventType) {
    if (this.listening[eventType]) return;
    this.listening[eventType] = true;
    this.eventSource.addEventListener(eventType, (event) => {
      this.notifyClients({
        type: eventType,
        data: event.data
      });
    });
  }

  notifyClients(event) {
    for (const client of this.clients) {
      client.postMessage(event);
    }
  }

  status(port) {
    port.postMessage({
      type: 'status',
      message: `url: ${this.url} readyState: ${this.eventSource.readyState}`,
    });
  }
}

self.onconnect = (e) => {
  for (const port of e.ports) {
    port.addEventListener('message', (event) => {
      if (event.data.type === 'start') {
        const url = event.data.url;
        if (sourcesByUrl[url]) {
          // we have a Source registered to this url
          const source = sourcesByUrl[url];
          source.register(port);
          sourcesByPort[port] = source;
          return;
        }
        let source = sourcesByPort[port];
        if (source) {
          if (source.eventSource && source.url === url) return;

          // How this has happened I don't understand...
          // deregister from that source
          const count = source.deregister(port);
          // Clean-up
          if (count === 0) {
            source.close();
            sourcesByUrl[source.url] = null;
          }
        }
        // Create a new Source
        source = new Source(url);
        source.register(port);
        sourcesByUrl[url] = source;
        sourcesByPort[port] = source;
      } else if (event.data.type === 'listen') {
        const source = sourcesByPort[port];
        source.listen(event.data.eventType);
      } else if (event.data.type === 'close') {
        const source = sourcesByPort[port];

        if (!source) return;

        const count = source.deregister(port);
        if (count === 0) {
          source.close();
          sourcesByUrl[source.url] = null;
          sourcesByPort[port] = null;
        }
      } else if (event.data.type === 'status') {
        const source = sourcesByPort[port];
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
};
