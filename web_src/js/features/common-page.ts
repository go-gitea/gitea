import {GET} from '../modules/fetch.ts';
import {showGlobalErrorMessage} from '../bootstrap.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {queryElems} from '../utils/dom.ts';
import {registerGlobalInitFunc, registerGlobalSelectorFunc} from '../modules/observer.ts';

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
  document.querySelector('.ui.dropdown .menu.language-menu')?.addEventListener('click', async (e) => {
    const item = (e.target as HTMLElement).closest('.item');
    if (!item) return;
    e.preventDefault();
    await GET(item.getAttribute('data-url'));
    window.location.reload();
  });
}

export function initGlobalDropdown() {
  // do not init "custom" dropdowns, "custom" dropdowns are managed by their own code.
  registerGlobalSelectorFunc('.ui.dropdown:not(.custom)', (el) => {
    const $dropdown = fomanticQuery(el);
    if ($dropdown.data('module-dropdown')) return; // do not re-init if other code has already initialized it.

    $dropdown.dropdown('setting', {hideDividers: 'empty'});

    if (el.classList.contains('jump')) {
      // The "jump" means this dropdown is mainly used for "menu" purpose,
      // clicking an item will jump to somewhere else or trigger an action/function.
      // When a dropdown is used for non-refresh actions with tippy,
      // it must have this "jump" class to hide the tippy when dropdown is closed.
      $dropdown.dropdown('setting', {
        action: 'hide',
        onShow() {
          // hide associated tooltip while dropdown is open
          this._tippy?.hide();
          this._tippy?.disable();
        },
        onHide() {
          this._tippy?.enable();
          // eslint-disable-next-line unicorn/no-this-assignment
          const elDropdown = this;

          // hide all tippy elements of items after a while. eg: use Enter to click "Copy Link" in the Issue Context Menu
          setTimeout(() => {
            const $dropdown = fomanticQuery(elDropdown);
            if ($dropdown.dropdown('is hidden')) {
              queryElems(elDropdown, '.menu > .item', (el) => el._tippy?.hide());
            }
          }, 2000);
        },
      });
    }

    // Special popup-directions, prevent Fomantic from guessing the popup direction.
    // With default "direction: auto", if the viewport height is small, Fomantic would show the popup upward,
    //   if the dropdown is at the beginning of the page, then the top part would be clipped by the window view.
    //   eg: Issue List "Sort" dropdown
    // But we can not set "direction: downward" for all dropdowns, because there is a bug in dropdown menu positioning when calculating the "left" position,
    //   which would make some dropdown popups slightly shift out of the right viewport edge in some cases.
    //   eg: the "Create New Repo" menu on the navbar.
    if (el.classList.contains('upward')) $dropdown.dropdown('setting', 'direction', 'upward');
    if (el.classList.contains('downward')) $dropdown.dropdown('setting', 'direction', 'downward');
  });
}

export function initGlobalTabularMenu() {
  fomanticQuery('.ui.menu.tabular:not(.custom) .item').tab();
}

// for performance considerations, it only uses performant syntax
function attachInputDirAuto(el: Partial<HTMLInputElement | HTMLTextAreaElement>) {
  if (el.type !== 'hidden' &&
    el.type !== 'checkbox' &&
    el.type !== 'radio' &&
    el.type !== 'range' &&
    el.type !== 'color') {
    el.dir = 'auto';
  }
}

export function initGlobalInput() {
  registerGlobalSelectorFunc('input, textarea', attachInputDirAuto);
  registerGlobalInitFunc('initInputAutoFocusEnd', (el: HTMLInputElement) => {
    el.focus(); // expects only one such element on one page. If there are many, then the last one gets the focus.
    el.setSelectionRange(el.value.length, el.value.length);
  });
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
