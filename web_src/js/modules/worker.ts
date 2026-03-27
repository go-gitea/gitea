const {appSubUrl, assetVersionEncoded} = window.config;

class UserEventsSharedWorker {
  sharedWorker: SharedWorker;

  constructor(worker: SharedWorker) {
    this.sharedWorker = worker;
    worker.addEventListener('error', (event) => {
      console.error('worker error', event);
    });
    worker.port.addEventListener('messageerror', () => {
      console.error('unable to deserialize message');
    });
    worker.port.postMessage({
      type: 'start',
      url: `${window.location.origin}${appSubUrl}/user/events`,
    });
    worker.port.addEventListener('error', (e) => {
      console.error('worker port error', e);
    });
    window.addEventListener('beforeunload', () => {
      worker.port.postMessage({type: 'close'});
      worker.port.close();
    });
  }

  addMessageEventListener(listener: (event: MessageEvent) => void) {
    this.sharedWorker.port.addEventListener('message', (event: MessageEvent) => {
      if (!event.data || !event.data.type) {
        console.error('unknown worker message event', event);
        return;
      }

      if (event.data.type === 'error') {
        console.error('worker port event error', event.data);
      } else if (event.data.type === 'logout') {
        if (event.data.data !== 'here') return;
        this.sharedWorker.port.postMessage({type: 'close'});
        this.sharedWorker.port.close();
        window.location.href = `${appSubUrl}/`;
      } else if (event.data.type === 'close') {
        this.sharedWorker.port.postMessage({type: 'close'});
        this.sharedWorker.port.close();
      }
      listener(event);
    });
  }

  startPort() {
    this.sharedWorker.port.start();
  }
}

export function initUserEventsSharedWorker(options: string) : UserEventsSharedWorker {
  const sharedWorker = new SharedWorker(`${window.__webpack_public_path__}js/eventsource.sharedworker.js?v=${assetVersionEncoded}`, options);
  return new UserEventsSharedWorker(sharedWorker);
}
