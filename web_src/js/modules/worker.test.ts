import type {UserEventMessage, WorkerInboundMessage} from '../types.ts';

// Minimal SharedWorker/MessagePort doubles: worker.ts wires user events onto
// `sharedWorker.port`, so we capture the port to feed it messages and assert dispatch behavior.
type PortListener = (ev: {data: WorkerInboundMessage}) => void;

class MockMessagePort {
  listeners: Record<string, PortListener[]> = {};
  posted: unknown[] = [];
  addEventListener(type: string, cb: PortListener) {
    (this.listeners[type] ||= []).push(cb);
  }
  removeEventListener() {}
  postMessage(msg: unknown) { this.posted.push(msg) }
  start() {}
  close() {}
  // Simulate the underlying worker delivering a message to the page.
  deliver(msg: UserEventMessage) {
    for (const cb of this.listeners['message'] || []) cb({data: {msgType: 'user-event', msgData: msg}});
  }
}

let lastWorker: MockSharedWorker;

class MockSharedWorker {
  port = new MockMessagePort();
  // eslint-disable-next-line unicorn/no-this-assignment -- the test needs a handle on the instance the module constructs
  constructor() { lastWorker = this }
  addEventListener() {}
}

// worker.ts caches module-scope state (subscribers, initialized), so re-import
// a fresh module per test after stubbing the globals it reads on init.
async function freshWorker() {
  vi.resetModules();
  vi.stubGlobal('WebSocket', class {});
  vi.stubGlobal('SharedWorker', MockSharedWorker);
  return await import('./worker.ts');
}

afterEach(() => {
  vi.unstubAllGlobals();
});

// sequential: freshWorker resets the module registry and stubs globals, which is unsafe
// to interleave with the other test under the repo's `sequence.concurrent` vitest config.
// Every push follows a real DB write, so a repeated value still means the
// underlying list changed (e.g. a new comment on an already-unread issue).
test('dispatches every push, including repeated values', {concurrent: false}, async () => {
  const {onUserEvent} = await freshWorker();
  const received: number[] = [];
  onUserEvent('notification-count', (msg) => { received.push(msg.eventData.count) });

  lastWorker.port.deliver({eventType: 'notification-count', eventData: {count: 1}});
  lastWorker.port.deliver({eventType: 'notification-count', eventData: {count: 1}});

  expect(received).toEqual([1, 1]);
});

test('worker-connected flags the page and reaches its subscribers', {concurrent: false}, async () => {
  const {onUserEvent} = await freshWorker();
  let connects = 0;
  onUserEvent('worker-connected', () => { connects++ });

  lastWorker.port.deliver({eventType: 'worker-connected'});

  expect(connects).toBe(1);
  expect(document.documentElement.getAttribute('data-user-events-connected')).toBe('true');
});
