const sourcesByUrl = {};
const sourcesByPort = {};

class Source {
  constructor(url) {
    this.url = url.replace(/^http/, 'ws');
    this.webSocket = new WebSocket(this.url);
    this.listening = {};
    this.clients = [];
    this.listen('open');
    this.listen('close');
    this.listen('logout');
    this.listen('notification-count');
    this.listen('stopwatches');
    this.listen('error');
    this.webSocket.addEventListener('error', (error) => {
      this.lastError = error;
    });
    this.webSocket.addEventListener('message', (event) => {
      const message = JSON.parse(event.data);
      if (!message) {
        return;
      }
      if (this.listening[message.Name]) {
        this.notifyClients({
          type: message.Name,
          data: message.Data
        });
      }
    });
    this.webSocket.addEventListener('close', (event) => {
      if (!this.webSocket) {
        return;
      }
      const oldWebSocket = this.webSocket;
      this.webSocket = null;
      this.notifyClients({
        type: 'close',
        data: event
      });
      oldWebSocket.close();
    });
  }

  register(port) {
    if (this.clients.includes(port)) return;

    this.clients.push(port);

    port.postMessage({
      type: 'status',
      message: `registered to ${this.url}`,
    });

    if (!this.webSocket) {
      if (this.lastError) {
        port.postMessage({
          type: 'error',
          message: `websocket disconnected: ${this.lastError}`
        });
      } else {
        port.postMessage({
          type: 'error',
          message: 'websocket disconnected'
        });
      }
    }
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
    if (!this.webSocket) return;
    const oldWebSocket = this.webSocket;
    this.webSocket = null;
    oldWebSocket.close();
  }

  listen(eventType) {
    if (this.listening[eventType]) return;
    this.listening[eventType] = true;
  }

  notifyClients(event) {
    for (const client of this.clients) {
      client.postMessage(event);
    }
  }

  status(port) {
    port.postMessage({
      type: 'status',
      message: `url: ${this.url} readyState: ${this.webSocket.readyState}`,
    });
  }
}

self.addEventListener('connect', (e) => {
  for (const port of e.ports) {
    port.addEventListener('message', (event) => {
      if (event.data.type === 'start') {
        const url = event.data.url;
        if (sourcesByUrl[url]) {
          // we have a Source registered to this url
          const source = sourcesByUrl[url];
          if (source.webSocket) {
            source.register(port);
            sourcesByPort[port] = source;
            return;
          }
          sourcesByUrl[url] = null;
        }
        let source = sourcesByPort[port];
        if (source) {
          if (source.webSocket && source.url === url) return;

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
});
