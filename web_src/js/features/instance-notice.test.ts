import {beforeEach, describe, expect, test, vi} from 'vitest';
import {initInstanceNotice} from './instance-notice.ts';
import {localUserSettings} from '../modules/user-settings.ts';

vi.mock('../modules/user-settings.ts', () => ({
  localUserSettings: {
    getString: vi.fn(),
    setString: vi.fn(),
  },
}));

function createBannerDOM(dismissKey: string) {
  document.body.innerHTML = `
    <div id="instance-notice-banner" class="ui info attached message" data-dismiss-key="${dismissKey}">
      <div class="tw-grid tw-grid-cols-[auto_1fr_auto] tw-items-center tw-gap-3">
        <div class="ui mini icon button tw-invisible tw-pointer-events-none tw-m-0" aria-hidden="true">X</div>
        <div class="render-content markup tw-text-center">Maintenance in progress</div>
        <button type="button" class="ui mini icon button instance-notice-dismiss tw-m-0">X</button>
      </div>
    </div>
  `;
}

describe('Instance Notice Banner Dismiss', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    document.body.innerHTML = '';
  });

  test('no banner in DOM does nothing', () => {
    initInstanceNotice();
    expect(localUserSettings.getString).not.toHaveBeenCalled();
  });

  test('hides banner when dismiss key matches stored value', () => {
    createBannerDOM('abc123');
    vi.mocked(localUserSettings.getString).mockReturnValue('abc123');

    initInstanceNotice();

    const banner = document.querySelector<HTMLElement>('#instance-notice-banner')!;
    expect(banner.style.display).toBe('none');
    expect(localUserSettings.setString).not.toHaveBeenCalled();
  });

  test('does not hide banner when stored key differs', () => {
    createBannerDOM('abc123');
    vi.mocked(localUserSettings.getString).mockReturnValue('old-key');

    initInstanceNotice();

    const banner = document.querySelector<HTMLElement>('#instance-notice-banner')!;
    expect(banner.style.display).not.toBe('none');
  });

  test('clicking dismiss button stores key and hides banner', () => {
    createBannerDOM('abc123');
    vi.mocked(localUserSettings.getString).mockReturnValue('');

    initInstanceNotice();

    const banner = document.querySelector<HTMLElement>('#instance-notice-banner')!;
    expect(banner.style.display).not.toBe('none');

    const dismissBtn = banner.querySelector<HTMLButtonElement>('.instance-notice-dismiss')!;
    dismissBtn.click();

    expect(localUserSettings.setString).toHaveBeenCalledWith('instance_notice_dismissed', 'abc123');
    expect(banner.style.display).toBe('none');
  });

  test('banner without data-dismiss-key does nothing', () => {
    document.body.innerHTML = `
      <div id="instance-notice-banner" class="ui info attached message">
        <div>Some message</div>
      </div>
    `;

    initInstanceNotice();

    const banner = document.querySelector<HTMLElement>('#instance-notice-banner')!;
    expect(banner.style.display).not.toBe('none');
    expect(localUserSettings.getString).not.toHaveBeenCalled();
  });
});
