import $ from 'jquery';
import {GET} from '../modules/fetch.ts';
import {showGlobalErrorMessage} from '../bootstrap.ts';

const {appUrl} = window.config;

export function initHeadNavbarContentToggle() {
  const navbar = document.querySelector('#navbar');
  const btn = document.querySelector('#navbar-expand-toggle');
  if (!navbar || !btn) return;

  btn.addEventListener('click', () => {
    const isExpanded = btn.classList.contains('active');
    navbar.classList.toggle('navbar-menu-open', !isExpanded);
    btn.classList.toggle('active', !isExpanded);
  });
}

export function initFootLanguageMenu() {
  async function linkLanguageAction() {
    const $this = $(this);
    await GET($this.data('url'));
    window.location.reload();
  }

  $('.language-menu a[lang]').on('click', linkLanguageAction);
}

export function initGlobalDropdown() {
  // Semantic UI modules.
  const $uiDropdowns = $('.ui.dropdown');

  // do not init "custom" dropdowns, "custom" dropdowns are managed by their own code.
  $uiDropdowns.filter(':not(.custom)').dropdown();

  // The "jump" means this dropdown is mainly used for "menu" purpose,
  // clicking an item will jump to somewhere else or trigger an action/function.
  // When a dropdown is used for non-refresh actions with tippy,
  // it must have this "jump" class to hide the tippy when dropdown is closed.
  $uiDropdowns.filter('.jump').dropdown({
    action: 'hide',
    onShow() {
      // hide associated tooltip while dropdown is open
      this._tippy?.hide();
      this._tippy?.disable();
    },
    onHide() {
      this._tippy?.enable();

      // hide all tippy elements of items after a while. eg: use Enter to click "Copy Link" in the Issue Context Menu
      setTimeout(() => {
        const $dropdown = $(this);
        if ($dropdown.dropdown('is hidden')) {
          $(this).find('.menu > .item').each((_, item) => {
            item._tippy?.hide();
          });
        }
      }, 2000);
    },
  });

  // Special popup-directions, prevent Fomantic from guessing the popup direction.
  // With default "direction: auto", if the viewport height is small, Fomantic would show the popup upward,
  //   if the dropdown is at the beginning of the page, then the top part would be clipped by the window view.
  //   eg: Issue List "Sort" dropdown
  // But we can not set "direction: downward" for all dropdowns, because there is a bug in dropdown menu positioning when calculating the "left" position,
  //   which would make some dropdown popups slightly shift out of the right viewport edge in some cases.
  //   eg: the "Create New Repo" menu on the navbar.
  $uiDropdowns.filter('.upward').dropdown('setting', 'direction', 'upward');
  $uiDropdowns.filter('.downward').dropdown('setting', 'direction', 'downward');
}

export function initGlobalTabularMenu() {
  $('.ui.menu.tabular:not(.custom) .item').tab({autoTabActivation: false});
}

/**
 * Too many users set their ROOT_URL to wrong value, and it causes a lot of problems:
 *   * Cross-origin API request without correct cookie
 *   * Incorrect href in <a>
 *   * ...
 * So we check whether current URL starts with AppUrl(ROOT_URL).
 * If they don't match, show a warning to users.
 */
export function checkAppUrl() {
  const curUrl = window.location.href;
  // some users visit "https://domain/gitea" while appUrl is "https://domain/gitea/", there should be no warning
  if (curUrl.startsWith(appUrl) || `${curUrl}/` === appUrl) {
    return;
  }
  showGlobalErrorMessage(`Your ROOT_URL in app.ini is "${appUrl}", it's unlikely matching the site you are visiting.
Mismatched ROOT_URL config causes wrong URL links for web UI/mail content/webhook notification/OAuth2 sign-in.`, 'warning');
}

export function checkAppUrlScheme() {
  const curUrl = window.location.href;
  // some users visit "http://domain" while appUrl is "https://domain", COOKIE_SECURE makes it impossible to sign in
  if (curUrl.startsWith('http:') && appUrl.startsWith('https:')) {
    showGlobalErrorMessage(`This instance is configured to run under HTTPS (by ROOT_URL config), you are accessing by HTTP. Mismatched scheme might cause problems for sign-in/sign-up.`, 'warning');
  }
}
