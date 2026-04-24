import type {WorkerInboundMessage} from './user-events-types.ts';

const {appSubUrl, sharedWorkerUri} = window.config;

export class UserEventsSharedWorker {
  sharedWorker: SharedWorker | null = null;
  private listeners: Array<(event: MessageEvent) => void> = [];
  private fallbackSignalled = false;

  constructor(options?: string | WorkerOptions) {
    const workerOptions: WorkerOptions = typeof options === 'string' ? {name: options} : {...options};
    workerOptions.type = 'module';
    let worker: SharedWorker;
    try {
      worker = new SharedWorker(sharedWorkerUri, workerOptions);
    } catch (err) {
      console.warn('SharedWorker unavailable, falling back to periodic polling', err);
      queueMicrotask(() => this.signalFallback());
      return;
    }
    this.sharedWorker = worker;
    // Browsers that reject the module `import` at parse time (no module-SharedWorker
    // support) fail here before the WebSocket ever opens — degrade to polling.
    worker.addEventListener('error', (event) => {
      console.error('worker error', event);
      this.signalFallback();
    });
    worker.port.addEventListener('messageerror', () => {
      console.error('unable to deserialize message');
    });
    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    worker.port.postMessage({
      type: 'start',
      url: `${wsProtocol}//${window.location.host}${appSubUrl}/-/ws`,
    });
    worker.port.addEventListener('error', (e) => {
      console.error('worker port error', e);
    });
    window.addEventListener('beforeunload', () => {
      // FIXME: this logic is not quite right.
      // "beforeunload" can be canceled by some actions like "are-you-sure" and the navigation can be cancelled.
      // In this case: the worker port is incorrectly closed while the page is still there.
      worker.port.postMessage({type: 'close'});
      worker.port.close();
    });
  }

  addMessageEventListener(listener: (event: MessageEvent) => void) {
    this.listeners.push(listener);
    this.sharedWorker?.port.addEventListener('message', (event: MessageEvent<WorkerInboundMessage>) => {
      if (!event.data || !event.data.type) {
        console.error('unknown worker message event', event);
        return;
      }

      if (event.data.type === 'error') {
        console.error('worker port event error', event.data);
      } else if (event.data.type === 'logout') {
        if (event.data.data !== 'here') return;
        this.sharedWorker!.port.postMessage({type: 'close'});
        this.sharedWorker!.port.close();
        // slightly delay our "logout" for a short while, in case there are other logout requests in-flight.
        // * if the logout is triggered by a page redirection (e.g.: user clicks "/user/logout")
        //   * "beforeunload" event is triggered, this code path won't execute
        // * if the logout is triggered by a fetch call
        //   * "beforeunload" event is not triggered until JS does the redirection.
        //     * in this case, the logout fetch call already completes and has sent the "logout" message to the worker
        //   * there can be a data-race between the fetch call's redirection and the "logout" message from the worker
        //     * the fetch call's logout redirection should always win over the worker message, because it might have a custom location
        setTimeout(() => { window.location.href = `${appSubUrl}/` }, 1000);
      } else if (event.data.type === 'close') {
        this.sharedWorker!.port.postMessage({type: 'close'});
        this.sharedWorker!.port.close();
      }
      listener(event);
    });
  }

  startPort() {
    this.sharedWorker?.port.start();
  }

  private signalFallback() {
    if (this.fallbackSignalled) return;
    this.fallbackSignalled = true;
    const syntheticEvent = {data: {type: 'push-unavailable'} satisfies WorkerInboundMessage} as MessageEvent;
    for (const listener of this.listeners) listener(syntheticEvent);
  }
}
