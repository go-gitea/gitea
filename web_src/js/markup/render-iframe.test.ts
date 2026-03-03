import {navigateToIframeLink, safeLinkHref} from './render-iframe.ts';

test('safeLinkHref', () => {
  expect(safeLinkHref('http://example.com')).toBe('http://example.com/');
  expect(safeLinkHref('https://example.com')).toBe('https://example.com/');
  expect(safeLinkHref('/path')).toBe('http://localhost:3000/path');
  // eslint-disable-next-line no-script-url
  expect(safeLinkHref('javascript:void(0);')).toBeNull();
  expect(safeLinkHref('data:image/svg+xml;utf8,<svg></svg>')).toBeNull();
  // for safety and consistency, it converts non-string input to string, just like `window.location.href = 0`
  expect(safeLinkHref(0)).toBe('http://localhost:3000/0');
  expect(safeLinkHref({})).toBe('http://localhost:3000/[object%20Object]');
  expect(safeLinkHref(null)).toBe('http://localhost:3000/null');
});

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

    // input can be any type & any value, keep the same behavior as `window.location.href = 123`
    navigateToIframeLink(123, {});
    expect(assignSpy).toHaveBeenCalledWith('http://localhost:3000/123');
    vi.clearAllMocks();
  });

  test('unsafe links', () => {
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
    vi.clearAllMocks();
  });
});
