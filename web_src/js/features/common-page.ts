import {GET, POST} from '../modules/fetch.ts';
import {showGlobalErrorMessage} from '../modules/errors.ts';
import {initDropdown} from '../modules/dropdown.ts';
import {initTabSwitcher} from '../modules/tab.ts';
import {addDelegatedEventListener, createElementFromHTML} from '../utils/dom.ts';
import {registerGlobalInitFunc, registerGlobalSelectorFunc} from '../modules/observer.ts';
import {initAvatarUploaderWithCropper} from './comp/Cropper.ts';
import {initCompSearchRepoBox} from './comp/SearchRepoBox.ts';
import {initRepoSwitcher} from './repo-switcher.ts';
import {initScopedWorkflowRequired} from './comp/ScopedWorkflows.ts';

const {appUrl, appSubUrl} = window.config;

function initHeadNavbarContentToggle() {
  const navbar = document.querySelector('#navbar');
  const btn = document.querySelector('#navbar-expand-toggle');
  if (!navbar || !btn) return;

  btn.addEventListener('click', () => {
    const isExpanded = btn.classList.contains('active');
    navbar.classList.toggle('navbar-menu-open', !isExpanded);
    btn.classList.toggle('active', !isExpanded);
  });
}

function initFooterLanguageMenu() {
  addDelegatedEventListener(document, 'click', '.footer-language-menu .dropdown-item', async (el) => {
    await GET(el.getAttribute('data-url')!);
    window.location.reload();
  });
}

function initFooterThemeSelector() {
  const elDropdown = document.querySelector<HTMLElement>('#footer-theme-selector');
  if (!elDropdown) return; // some pages don't have footer, for example: 500.tmpl
  const dropdown = initDropdown(elDropdown, {
    async loadItems(_query, signal) {
      const resp = await GET(`${appSubUrl}/-/web-theme/list`, {signal});
      const {results} = await resp.json() as {results: Array<{name: string, value: string, active: boolean}>};
      return results.map(({name, value, active}) => {
        const item = createElementFromHTML(name);
        item.classList.add('dropdown-item');
        item.classList.toggle('active', active);
        item.setAttribute('data-value', value);
        return item;
      });
    },
  });
  addDelegatedEventListener(dropdown.popup, 'click', '.dropdown-item', async (el) => {
    await POST(`${appSubUrl}/-/web-theme/apply?theme=${encodeURIComponent(el.getAttribute('data-value')!)}`);
    window.location.reload();
  });
}

export function initCommmPageComponents() {
  initHeadNavbarContentToggle();
  initFooterLanguageMenu();
  initFooterThemeSelector();
}

export function initGlobalDropdown() {
  registerGlobalSelectorFunc('.dropdown-container', (el: HTMLElement) => initDropdown(el));
}

export function initGlobalComponent() {
  registerGlobalInitFunc('initTabSwitcher', initTabSwitcher);
  registerGlobalInitFunc('initAvatarUploader', initAvatarUploaderWithCropper);
  registerGlobalInitFunc('initSearchRepoBox', initCompSearchRepoBox);
  registerGlobalInitFunc('initRepoSwitcher', initRepoSwitcher);
  registerGlobalInitFunc('initScopedWorkflowRequired', initScopedWorkflowRequired);
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

function autoFocusEnd(el: HTMLInputElement | HTMLTextAreaElement) {
  el.focus();
  el.setSelectionRange(el.value.length, el.value.length);
}

export function applyAutoFocus(container: Element) {
  // https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/autofocus
  // "autofocus" behavior is defined by the standard: when a container (e.g.: dialog) becomes visible, focus the element with "autofocus" attribute
  // The native modal dialog already supports it, we need to cover more cases (e.g.: ".show-panel" button)
  // Here is just a simple support, we don't expect more than one element that need "autofocus" appearing in the same container
  container.querySelector<HTMLElement>('[autofocus]')?.focus();
  // Also, apply our autoFocusEnd behavior
  // TODO: GLOBAL-INIT-MULTIPLE-FUNCTIONS: use "~=" operator in case we would extend the "data-global-init" to support more functions in the future.
  const el = container.querySelector<HTMLInputElement>('[data-global-init~="autoFocusEnd"]');
  if (el) autoFocusEnd(el);
}

export function initGlobalInput() {
  registerGlobalSelectorFunc('input, textarea', attachInputDirAuto);

  // autoFocusEnd is used for autofocus an input/textarea and move the cursor to the end of the text.
  // It is useful for "New Issue"/"New PR" pages when the title is pre-filled with prefix text (e.g.: from template or commit message)
  // The native "autofocus" isn't used because there is a delay between "focused (DOM rendering)" and "move cursor to end (our JS)", it causes flickers.
  registerGlobalInitFunc('autoFocusEnd', autoFocusEnd);
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
  showGlobalErrorMessage(`The detected web site URL is "${appUrl}", it's unlikely matching the site config.
Mismatched app.ini ROOT_URL or reverse proxy "Host/X-Forwarded-Proto" config might cause wrong URL links for web UI/mail content/webhook notification/OAuth2 sign-in.`, 'warning');
}

export function checkAppUrlScheme() {
  const curUrl = window.location.href;
  // some users visit "http://domain" while appUrl is "https://domain", COOKIE_SECURE makes it impossible to sign in
  if (curUrl.startsWith('http:') && appUrl.startsWith('https:')) {
    showGlobalErrorMessage(`This instance is configured to run under HTTPS (by ROOT_URL config), you are accessing by HTTP. Mismatched scheme might cause problems for sign-in/sign-up.`, 'warning');
  }
}
