type HashChangeListener = (e: HashChangeEvent) => void | Promise<void>;

export type HashChangeListenerOptions = {
  priority: number;
};

const hashChangeListeners: Array<{
  opts: HashChangeListenerOptions;
  listener: HashChangeListener;
}> = [];

export function registerAsyncHashChangeListener(listener: HashChangeListener, priority: number = 0): void {
  hashChangeListeners.push({opts: {priority}, listener});
  hashChangeListeners.sort((a, b) => a.opts.priority - b.opts.priority);
}

export function removeAsyncHashChangeListener(listener: HashChangeListener): void {
  const index = hashChangeListeners.findIndex((item) => item.listener === listener);
  if (index !== -1) hashChangeListeners.splice(index, 1);
}

export async function initAsyncHashChangeListeners() {
  for (const {listener} of hashChangeListeners) {
    await listener(new HashChangeEvent('hashchange'));
  }
  window.addEventListener('hashchange', async (e) => {
    for (const {listener} of hashChangeListeners) await listener(e);
  });
}
