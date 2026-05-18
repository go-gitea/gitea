import type {UserEventMessage, UserEventType, WorkerInboundMessage} from '../types.ts';

const {appSubUrl, sharedWorkerUri} = window.config;

type EventOf<T extends UserEventType> = Extract<UserEventMessage, {type: T}>;
type Subscriber<T extends UserEventType = UserEventType> = (msg: EventOf<T>) => void;
const subscribers = new Map<UserEventType, Set<Subscriber>>();
// Suppress identical repeat pushes (Redis fan-out can emit dup counts) so subscribers don't refetch on no-ops.
const lastPayload = new Map<UserEventType, string>();
let fallbackSignalled = false;
let sharedWorker: SharedWorker | null = null;

function dispatch(msg: UserEventMessage) {
  const serialized = JSON.stringify(msg);
  if (lastPayload.get(msg.type) === serialized) return;
  lastPayload.set(msg.type, serialized);
  const set = subscribers.get(msg.type);
  if (!set) return;
  for (const cb of set) cb(msg);
}

function signalFallback() {
  if (fallbackSignalled) return;
  fallbackSignalled = true;
  dispatch({type: 'push-unavailable'});
}

function init() {
  try {
    sharedWorker = new SharedWorker(sharedWorkerUri, {type: 'module', name: 'user-events'});
  } catch (err) {
    console.warn('SharedWorker unavailable, falling back to periodic polling', err);
    queueMicrotask(signalFallback);
    return;
  }
  // Browsers without module-SharedWorker support fail at parse time, before the WebSocket opens.
  sharedWorker.addEventListener('error', (event) => {
    console.error('worker error', event);
    signalFallback();
  });
  sharedWorker.port.addEventListener('messageerror', () => {
    console.error('unable to deserialize message');
  });
  sharedWorker.port.addEventListener('error', (e) => {
    console.error('worker port error', e);
  });
  sharedWorker.port.addEventListener('message', (event: MessageEvent<WorkerInboundMessage>) => {
    const msg = event.data;
    if (!msg || !msg.type) {
      console.error('unknown worker message event', event);
      return;
    }
    if (msg.type === 'error') {
      console.error('worker port event error', msg);
      return;
    }
    if (msg.type === 'close') {
      sharedWorker!.port.postMessage({type: 'close'});
      sharedWorker!.port.close();
      return;
    }
    if (msg.type === 'logout') {
      if (msg.data !== 'here') return;
      sharedWorker!.port.postMessage({type: 'close'});
      sharedWorker!.port.close();
      // slightly delay our "logout" for a short while, in case there are other logout requests in-flight.
      // * if the logout is triggered by a page redirection (e.g.: user clicks "/user/logout")
      //   * "beforeunload" event is triggered, this code path won't execute
      // * if the logout is triggered by a fetch call
      //   * "beforeunload" event is not triggered until JS does the redirection.
      //     * in this case, the logout fetch call already completes and has sent the "logout" message to the worker
      //   * there can be a data-race between the fetch call's redirection and the "logout" message from the worker
      //     * the fetch call's logout redirection should always win over the worker message, because it might have a custom location
      setTimeout(() => { window.location.href = `${appSubUrl}/` }, 1000);
    }
    dispatch(msg);
  });
  sharedWorker.port.start();
  const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  sharedWorker.port.postMessage({
    type: 'start',
    url: `${wsProtocol}//${window.location.host}${appSubUrl}/-/ws`,
  });
  window.addEventListener('beforeunload', () => {
    // FIXME: this logic is not quite right.
    // "beforeunload" can be canceled by some actions like "are-you-sure" and the navigation can be cancelled.
    // In this case: the worker port is incorrectly closed while the page is still there.
    sharedWorker!.port.postMessage({type: 'close'});
    sharedWorker!.port.close();
  });
}

let initialized = false;
export function onUserEvent<T extends UserEventType>(type: T, cb: Subscriber<T>): () => void {
  if (!initialized) {
    initialized = true;
    if (window.WebSocket && window.SharedWorker) {
      init();
    } else {
      queueMicrotask(signalFallback);
    }
  }
  let set = subscribers.get(type);
  if (!set) {
    set = new Set();
    subscribers.set(type, set);
  }
  const wrapped: Subscriber = (msg) => cb(msg as EventOf<T>);
  set.add(wrapped);
  return () => { set.delete(wrapped) };
}
