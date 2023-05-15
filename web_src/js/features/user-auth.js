import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';

export function initUserAuthOauth2() {
  const $oauth2LoginNav = $('#oauth2-login-navigator');
  if ($oauth2LoginNav.length === 0) return;

  $oauth2LoginNav.find('.oauth-login-image').on('click', () => {
    const oauthLoader = $('#oauth2-login-loader');
    const oauthNav = $('#oauth2-login-navigator');

    hideElem(oauthNav);
    oauthLoader.removeClass('disabled');

    setTimeout(() => {
      // recover previous content to let user try again
      // usually redirection will be performed before this action
      oauthLoader.addClass('disabled');
      showElem(oauthNav);
    }, 5000);
  });
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
