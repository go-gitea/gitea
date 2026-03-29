const {appSubUrl, sharedWorkerUri} = window.config;

export class UserEventsSharedWorker {
  sharedWorker: SharedWorker;

  // options can be either a string (the debug name of the worker) or an object of type WorkerOptions
  constructor(options?: string | WorkerOptions) {
    const worker = new SharedWorker(sharedWorkerUri, options);
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
      // FIXME: this logic is not quite right.
      // "beforeunload" can be canceled by some actions like "are-you-sure" and the navigation can be cancelled.
      // In this case: the worker port is incorrectly closed while the page is still there.
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
