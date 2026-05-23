import {navigateToIframeLink} from './render-iframe.ts';

describe('navigateToIframeLink', () => {
  const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
  const assignSpy = vi.spyOn(window.location, 'assign').mockImplementation(() => undefined);

  test('safe links', () => {
    navigateToIframeLink('http://example.com', '_blank');
    expect(openSpy).toHaveBeenCalledWith('http://example.com/', '_blank', 'noopener,noreferrer');
    vi.clearAllMocks();

    navigateToIframeLink('https://example.com', '_self');
    expect(assignSpy).toHaveBeenCalledWith('https://example.com/');
    vi.clearAllMocks();

    navigateToIframeLink('https://example.com', null);
    expect(assignSpy).toHaveBeenCalledWith('https://example.com/');
    vi.clearAllMocks();

    navigateToIframeLink('/path', '');
    expect(assignSpy).toHaveBeenCalledWith('http://localhost:3000/path');
    vi.clearAllMocks();

    // input can be any type & any value, keep the same behavior as `window.location.href = 0`
    navigateToIframeLink(0, {});
    expect(assignSpy).toHaveBeenCalledWith('http://localhost:3000/0');
    vi.clearAllMocks();
  });

  test('unsafe links', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    window.location.href = 'http://localhost:3000/';

    // eslint-disable-next-line no-script-url
    navigateToIframeLink('javascript:void(0);', '_blank');
    expect(openSpy).toHaveBeenCalledTimes(0);
    expect(assignSpy).toHaveBeenCalledTimes(0);
    expect(window.location.href).toBe('http://localhost:3000/');
    vi.clearAllMocks();

    navigateToIframeLink('data:image/svg+xml;utf8,<svg></svg>', '');
    expect(openSpy).toHaveBeenCalledTimes(0);
    expect(assignSpy).toHaveBeenCalledTimes(0);
    expect(window.location.href).toBe('http://localhost:3000/');
    errorSpy.mockRestore();
    vi.clearAllMocks();
  });
});
