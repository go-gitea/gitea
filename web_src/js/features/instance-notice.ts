import {localUserSettings} from '../modules/user-settings.ts';

const DISMISSED_KEY = 'instance_notice_dismissed';

function hideBanner(el: HTMLElement) {
  el.style.display = 'none';
}

export function initInstanceNotice(): void {
  const banner = document.querySelector<HTMLElement>('#instance-notice-banner');
  if (!banner) return;

  const dismissKey = banner.getAttribute('data-dismiss-key');
  if (!dismissKey) return;

  if (localUserSettings.getString(DISMISSED_KEY, '') === dismissKey) {
    hideBanner(banner);
    return;
  }

  const dismissBtn = banner.querySelector<HTMLButtonElement>('.instance-notice-dismiss');
  if (dismissBtn) {
    dismissBtn.addEventListener('click', () => {
      localUserSettings.setString(DISMISSED_KEY, dismissKey);
      hideBanner(banner);
    });
  }
}
