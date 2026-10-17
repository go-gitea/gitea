import {checkAppUrl, checkAppUrlScheme} from './common-page.ts';

export function initUserCheckAppUrl() {
  if (!document.querySelector('.page-content.user.signin, .page-content.user.signup, .page-content.user.link-account')) return;
  checkAppUrlScheme();
}

export function initUserExternalLogins() {
  const container = document.querySelector('#external-login-navigator');
  if (!container) return;

  // whether the auth method requires app url check (need consistent ROOT_URL with visited URL)
  let needCheckAppUrl = false;
  for (const link of container.querySelectorAll('.external-login-link')) {
    needCheckAppUrl = needCheckAppUrl || link.getAttribute('data-require-appurl-check') === 'true';
    link.addEventListener('click', () => {
      container.classList.add('is-loading');
      setTimeout(() => {
        // recover previous content to let user try again, usually redirection will be performed before this action
        container.classList.remove('is-loading');
      }, 5000);
    });
  }
  if (needCheckAppUrl) {
    checkAppUrl();
  }
}
