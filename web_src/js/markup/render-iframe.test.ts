import {navigateToIframeLink} from './render-iframe.ts';
import {navigateTo} from '../utils/testhelper.ts';

vi.mock('../utils/testhelper.ts', () => ({navigateTo: vi.fn()}));

describe('navigateToIframeLink', () => {
  const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);

  test('safe links', () => {
    navigateToIframeLink('http://example.com', '_blank');
    expect(openSpy).toHaveBeenCalledWith('http://example.com/', '_blank', 'noopener,noreferrer');
    vi.clearAllMocks();

    navigateToIframeLink('https://example.com', '_self');
    expect(navigateTo).toHaveBeenCalledWith('https://example.com/');
    vi.clearAllMocks();

    navigateToIframeLink('https://example.com', null);
    expect(navigateTo).toHaveBeenCalledWith('https://example.com/');
    vi.clearAllMocks();

    navigateToIframeLink('/path', '');
    expect(navigateTo).toHaveBeenCalledWith(`${window.location.origin}/path`);
    vi.clearAllMocks();

    // input can be any type & any value, keep the same behavior as `window.location.href = 0`
    navigateToIframeLink(0, {});
    expect(navigateTo).toHaveBeenCalledWith(`${window.location.origin}/0`);
    vi.clearAllMocks();
  });

  test('unsafe links', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

    // eslint-disable-next-line no-script-url
    navigateToIframeLink('javascript:void(0);', '_blank');
    expect(openSpy).toHaveBeenCalledTimes(0);
    expect(navigateTo).toHaveBeenCalledTimes(0);
    vi.clearAllMocks();

    navigateToIframeLink('data:image/svg+xml;utf8,<svg></svg>', '');
    expect(openSpy).toHaveBeenCalledTimes(0);
    expect(navigateTo).toHaveBeenCalledTimes(0);
    errorSpy.mockRestore();
    vi.clearAllMocks();
  });
});
