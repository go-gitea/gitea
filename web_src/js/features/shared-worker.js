const {appSubUrl} = window.config;

let worker;

function getMemoizedSharedWorker() {
  if (!window.EventSource || !window.SharedWorker) {
    return null;
  }

  if (!worker) {
    worker = new SharedWorker(
      `${__webpack_public_path__}js/eventsource.sharedworker.js`,
      'notification-worker'
    );

    worker.addEventListener('error', (event) => {
      console.error(event);
    });

    worker.port.addEventListener('messageerror', () => {
      console.error('Unable to deserialize message');
    });

    worker.port.postMessage({
      type: 'start',
      url: `${window.location.origin}${appSubUrl}/user/events`,
    });

    worker.port.addEventListener('message', (event) => {
      if (!event.data || !event.data.type) {
        console.error(event);
        return;
      }
      if (event.data.type === 'error') {
        console.error(event.data);
      } else if (event.data.type === 'logout') {
        if (event.data.data !== 'here') {
          return;
        }
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
        window.location.href = appSubUrl;
      } else if (event.data.type === 'close') {
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
      }
    });

    worker.port.addEventListener('error', (e) => {
      console.error(e);
    });

    worker.port.start();

    window.addEventListener('beforeunload', () => {
      worker.port.postMessage({
        type: 'close',
      });
      worker.port.close();
    });
  }

  return worker;
}

export default getMemoizedSharedWorker;
