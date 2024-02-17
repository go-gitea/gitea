import {checkAppUrl} from './common-global.js';

export function initUserAuthOauth2() {
  const outer = document.getElementById('oauth2-login-navigator');
  if (!outer) return;
  const inner = document.getElementById('oauth2-login-navigator-inner');

  checkAppUrl();

  for (const link of outer.querySelectorAll('.oauth-login-link')) {
    link.addEventListener('click', () => {
      inner.classList.add('gt-invisible');
      outer.classList.add('is-loading');
      setTimeout(() => {
        // recover previous content to let user try again
        // usually redirection will be performed before this action
        outer.classList.remove('is-loading');
        inner.classList.remove('gt-invisible');
      }, 5000);
    });
  }
}

export function initUserAuthLinkAccountView() {
  const lnkUserPage = document.querySelector('.page-content.user.link-account');
  if (!lnkUserPage) {
    return false;
  }

  const signinTab = lnkUserPage.querySelector('.item[data-tab="auth-link-signin-tab"]');
  const signUpTab = lnkUserPage.querySelector('.item[data-tab="auth-link-signup-tab"]');
  const signInView = lnkUserPage.querySelector('.tab[data-tab="auth-link-signin-tab"]');
  const signUpView = lnkUserPage.querySelector('.tab[data-tab="auth-link-signup-tab"]');

  signUpTab.addEventListener('click', (e) => {
    e.preventDefault();
    e.stopPropagation();
    signinTab.classList.remove('active');
    signInView.classList.remove('active');
    signUpTab.classList.add('active');
    signUpView.classList.add('active');
  });

  signinTab.addEventListener('click', () => {
    signUpTab.classList.remove('active');
    signUpView.classList.remove('active');
    signinTab.classList.add('active');
    signInView.classList.add('active');
  });
}
