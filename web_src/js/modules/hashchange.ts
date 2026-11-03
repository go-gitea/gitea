type HashChangeListener = (e: HashChangeEvent) => void | Promise<void>;

const hashChangeListeners: Array<{listener: HashChangeListener, priority: number}> = [];
let hashChangeListenersInited = false;

export function registerAsyncHashChangeListener(listener: HashChangeListener, priority = 0): void {
  hashChangeListeners.push({listener, priority});
  hashChangeListeners.sort((a, b) => a.priority - b.priority);
  if (hashChangeListenersInited) listener(new HashChangeEvent('hashchange')); // late registrants missed the initial run
}

export function removeAsyncHashChangeListener(listener: HashChangeListener): void {
  const index = hashChangeListeners.findIndex((item) => item.listener === listener);
  if (index !== -1) hashChangeListeners.splice(index, 1);
}

async function runHashChangeListeners(e: HashChangeEvent) {
  for (const {listener} of hashChangeListeners) await listener(e);
}

export async function initAsyncHashChangeListeners() {
  hashChangeListenersInited = true;
  window.addEventListener('hashchange', runHashChangeListeners);
  await runHashChangeListeners(new HashChangeEvent('hashchange'));
}
