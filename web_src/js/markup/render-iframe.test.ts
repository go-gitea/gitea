import {initMarkupRenderIframe} from './render-iframe.ts';

function setupIframe() {
  document.body.innerHTML = '<div id="root"><iframe class="external-render-iframe" data-src="/render"></iframe></div>';
  const root = document.getElementById('root')!;
  initMarkupRenderIframe(root);

  const iframe = root.querySelector<HTMLIFrameElement>('iframe.external-render-iframe')!;
  const iframeWindow = iframe.contentWindow ?? ({} as Window);
  if (!iframe.contentWindow) {
    Object.defineProperty(iframe, 'contentWindow', {value: iframeWindow});
  }

  return {iframe, iframeWindow};
}

function dispatchMessage(source: Window, data: Record<string, unknown>) {
  window.dispatchEvent(new MessageEvent('message', {data, source} as MessageEventInit));
}

beforeEach(() => {
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

test('open-link allows http/https and respects target', () => {
  const {iframe, iframeWindow} = setupIframe();
  const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
  const assignSpy = vi.spyOn(window.location, 'assign').mockImplementation(() => undefined);

  dispatchMessage(iframeWindow, {
    giteaIframeCmd: 'open-link',
    giteaIframeId: iframe.id,
    openLink: 'https://example.com/path',
    anchorTarget: '_blank',
  });

  expect(openSpy).toHaveBeenCalledWith('https://example.com/path', '_blank', 'noopener');
  expect(assignSpy).not.toHaveBeenCalled();

  openSpy.mockClear();
  assignSpy.mockClear();

  dispatchMessage(iframeWindow, {
    giteaIframeCmd: 'open-link',
    giteaIframeId: iframe.id,
    openLink: 'https://example.com/next',
    anchorTarget: '',
  });

  expect(openSpy).not.toHaveBeenCalled();
  expect(assignSpy).toHaveBeenCalledWith('https://example.com/next');
});

test('open-link rejects non-http schemes', () => {
  const {iframe, iframeWindow} = setupIframe();
  const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
  const assignSpy = vi.spyOn(window.location, 'assign').mockImplementation(() => undefined);

  dispatchMessage(iframeWindow, {
    giteaIframeCmd: 'open-link',
    giteaIframeId: iframe.id,
    openLink: 'javascript:alert(1)',
    anchorTarget: '',
  });

  expect(openSpy).not.toHaveBeenCalled();
  expect(assignSpy).not.toHaveBeenCalled();
});

test('open-link ignores messages from other windows', () => {
  const {iframe} = setupIframe();
  const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
  const assignSpy = vi.spyOn(window.location, 'assign').mockImplementation(() => undefined);

  dispatchMessage({} as Window, {
    giteaIframeCmd: 'open-link',
    giteaIframeId: iframe.id,
    openLink: 'https://example.com/path',
    anchorTarget: '_blank',
  });

  expect(openSpy).not.toHaveBeenCalled();
  expect(assignSpy).not.toHaveBeenCalled();
});
