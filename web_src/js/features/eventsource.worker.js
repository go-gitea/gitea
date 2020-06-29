let eventSource;

const listening = {};

self.addEventListener('message', (event) => {
  if (event.data.type === 'start') {
    eventSource = new EventSource(event.data.url);
    listen('open');
    listen('error');
    listen('notification-count');
    this.listen('logout');
  } else if (event.data.type === 'listen') {
    listen(event.data.eventType);
  } else if (event.data.type === 'close' && eventSource) {
    eventSource.close();
    eventSource = null;
  }
}, false);

function listen (eventType) {
  if (listening[eventType]) return;
  listening[eventType] = true;
  eventSource.addEventListener(eventType, (event) => {
    self.postMessage({
      type: eventType,
      data: event.data
    }, false);
  });
}
