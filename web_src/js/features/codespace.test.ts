import {initCodespaceLiveState} from './codespace.ts';

test('initCodespaceLiveState refreshes the state fragment', async () => {
  vi.useFakeTimers();
  try {
    document.body.innerHTML = `
      <div id="codespace-live-state" data-state-url="/-/codespaces/uuid/state" data-refresh-after-ms="10">old</div>
    `;
    const fetchMock = vi.fn().mockResolvedValue(new Response(`
      <div id="codespace-live-state" data-state-url="/-/codespaces/uuid/state" data-refresh-after-ms="20">new</div>
    `, {status: 200}));
    vi.stubGlobal('fetch', fetchMock);

    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(10);

    expect(fetchMock).toHaveBeenCalledWith('/-/codespaces/uuid/state', expect.objectContaining({method: 'GET'}));
    expect(document.querySelector('#codespace-live-state')?.textContent).toContain('new');
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});
