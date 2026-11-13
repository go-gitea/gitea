import {GET, POST, PATCH, PUT, DELETE, ExclusiveFetch} from './fetch.ts';

// This test satisfies the linter for unused functions.
test('exports', () => {
  expect(GET).toBeTruthy();
  expect(POST).toBeTruthy();
  expect(PATCH).toBeTruthy();
  expect(PUT).toBeTruthy();
  expect(DELETE).toBeTruthy();
});

test('ExclusiveFetch discards a superseded response while reading its body', async () => {
  const bodyStarted = Promise.withResolvers<void>();
  const oldBody = Promise.withResolvers<string>();
  const oldResponse = new Response();
  vi.spyOn(oldResponse, 'text').mockImplementation(() => {
    bodyStarted.resolve();
    return oldBody.promise;
  });
  const fetchMock = vi.spyOn(globalThis, 'fetch')
    .mockResolvedValueOnce(oldResponse)
    .mockResolvedValueOnce(new Response('new'));
  onTestFinished(() => { fetchMock.mockRestore() });

  const exclusiveFetch = new ExclusiveFetch();
  const render = vi.fn();
  const oldRequest = exclusiveFetch.request('/old').fetchText(render);
  expect(exclusiveFetch.isInFlight()).toBe(true);
  await bodyStarted.promise;
  await exclusiveFetch.request('/new').fetchText(render);
  expect(exclusiveFetch.isInFlight()).toBe(false);

  oldBody.resolve('old');
  await oldRequest;
  expect(render.mock.calls).toEqual([['new']]);
});
