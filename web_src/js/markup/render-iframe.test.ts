import {navigateToIframeLink} from './render-iframe.ts';
import {captureNavigations} from '../utils/testhelper.ts';

describe('navigateToIframeLink', () => {
  test('safe links', () => {
    const navigations = captureNavigations();
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
    navigateToIframeLink('http://example.com', '_blank');
    expect(openSpy).toHaveBeenCalledWith('http://example.com/', '_blank', 'noopener,noreferrer');
    navigateToIframeLink('https://example.com', '_self');
    expect(navigations.at(-1)!.url).toEqual('https://example.com/');
    navigateToIframeLink('https://example.com', null);
    expect(navigations.at(-1)!.url).toEqual('https://example.com/');
    navigateToIframeLink('/path', '');
    expect(navigations.at(-1)!.url).toEqual(`${window.location.origin}/path`);
    // input can be any type & any value, keep the same behavior as `window.location.href = 0`
    navigateToIframeLink(0, {});
    expect(navigations.at(-1)!.url).toEqual(`${window.location.origin}/0`);
    expect(navigations).toHaveLength(4);
    openSpy.mockRestore();
  });

  test('unsafe links', () => {
    const navigations = captureNavigations();
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    // eslint-disable-next-line no-script-url
    navigateToIframeLink('javascript:void(0);', '_blank');
    navigateToIframeLink('data:image/svg+xml;utf8,<svg></svg>', '');
    expect(openSpy).toHaveBeenCalledTimes(0);
    expect(navigations).toEqual([]);
    openSpy.mockRestore();
    errorSpy.mockRestore();
  });
});
