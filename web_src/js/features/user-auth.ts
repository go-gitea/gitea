import {checkAppUrl, checkAppUrlScheme} from './common-page.ts';

export function initUserCheckAppUrl() {
  if (!document.querySelector('.page-content.user.signin, .page-content.user.signup, .page-content.user.link-account')) return;
  checkAppUrlScheme();
}

export function initUserAuthOauth2() {
  const outer = document.querySelector('#oauth2-login-navigator');
  if (!outer) return;
  const inner = document.querySelector('#oauth2-login-navigator-inner');

  checkAppUrl();

  for (const link of outer.querySelectorAll('.oauth-login-link')) {
    link.addEventListener('click', () => {
      inner.classList.add('tw-invisible');
      outer.classList.add('is-loading');
      setTimeout(() => {
        // recover previous content to let user try again
        // usually redirection will be performed before this action
        outer.classList.remove('is-loading');
        inner.classList.remove('tw-invisible');
      }, 5000);
    });
  }
}
