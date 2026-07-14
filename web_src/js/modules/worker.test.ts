import type {UserEventMessage} from '../types.ts';

// Minimal SharedWorker/MessagePort doubles: worker.ts wires user events onto
// `sharedWorker.port`, so we capture the port to feed it messages and assert dispatch behaviour.
type PortListener = (ev: {data: unknown}) => void;

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
    for (const cb of this.listeners['message'] || []) cb({data: msg});
  }
}

let lastWorker: MockSharedWorker;

class MockSharedWorker {
  port = new MockMessagePort();
  // eslint-disable-next-line unicorn/no-this-assignment
  constructor() { lastWorker = this }
  addEventListener() {}
}

// worker.ts caches module-scope state (subscribers, lastPayload, initialized),
// so re-import a fresh module per test after stubbing the globals it reads on init.
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
test('dedups identical repeat pushes', {concurrent: false}, async () => {
  const {onUserEvent} = await freshWorker();
  const received: number[] = [];
  onUserEvent('notification-count', (msg) => { received.push(msg.count) });

  lastWorker.port.deliver({type: 'notification-count', count: 1});
  lastWorker.port.deliver({type: 'notification-count', count: 1}); // identical -> suppressed

  expect(received).toEqual([1]);
});

test('ws-connected clears the dedup cache so a repeat-value push dispatches again', {concurrent: false}, async () => {
  const {onUserEvent} = await freshWorker();
  const received: number[] = [];
  let connects = 0;
  onUserEvent('notification-count', (msg) => { received.push(msg.count) });
  onUserEvent('ws-connected', () => { connects++ });

  lastWorker.port.deliver({type: 'notification-count', count: 1});
  lastWorker.port.deliver({type: 'ws-connected'}); // must clear lastPayload
  lastWorker.port.deliver({type: 'notification-count', count: 1}); // same value, cache cleared -> delivered again

  expect(connects).toBe(1);
  expect(received).toEqual([1, 1]);
});
