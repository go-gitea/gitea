import $ from 'jquery';
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
  const $lnkUserPage = $('.page-content.user.link-account');
  if ($lnkUserPage.length === 0) {
    return false;
  }

  const $signinTab = $lnkUserPage.find('.item[data-tab="auth-link-signin-tab"]');
  const $signUpTab = $lnkUserPage.find('.item[data-tab="auth-link-signup-tab"]');
  const $signInView = $lnkUserPage.find('.tab[data-tab="auth-link-signin-tab"]');
  const $signUpView = $lnkUserPage.find('.tab[data-tab="auth-link-signup-tab"]');

  $signUpTab.on('click', () => {
    $signinTab.removeClass('active');
    $signInView.removeClass('active');
    $signUpTab.addClass('active');
    $signUpView.addClass('active');
    return false;
  });

  $signinTab.on('click', () => {
    $signUpTab.removeClass('active');
    $signUpView.removeClass('active');
    $signinTab.addClass('active');
    $signInView.addClass('active');
  });
}
