import {GET, request} from '../modules/fetch.ts';
import {hideToastsAll, showErrorToast} from '../modules/toast.ts';
import {addDelegatedEventListener, createElementFromHTML, submitEventSubmitter} from '../utils/dom.ts';
import {confirmModal, createConfirmModal} from './comp/ConfirmModal.ts';
import {ignoreAreYouSure} from '../vendor/jquery.are-you-sure.ts';
import {registerGlobalSelectorFunc} from '../modules/observer.ts';
import {Idiomorph} from 'idiomorph';
import {parseDom} from '../utils.ts';
import {html} from '../utils/html.ts';

const {appSubUrl, runModeIsProd} = window.config;

type FetchActionOpts = {
  method: string;
  url: string;
  headers?: HeadersInit;
  body?: FormData;

  // pseudo selectors/commands to update the current page with the response text when the response is text (html)
  // e.g.: "$this", "$innerHTML", "$closest(tr) td .the-class", "$body #the-id"
  successSync: string;

  // the loading indicator element selector, it uses the same syntax as "data-fetch-sync" to find the element(s)
  // empty means no loading indicator, "$this" means the element itself
  loadingIndicator: string;
};

// fetchActionDoRedirect does real redirection to bypass the browser's limitations of "location"
// more details are in the backend's fetch-redirect handler
function fetchActionDoRedirect(redirect: string) {
  // In production, if the link can be directly navigated by browser, we just do normal redirection, which is faster.
  // Otherwise, need to use backend to do redirection:
  // * Also do so in development, to make sure the redirection logic is always tested by real users
  const needBackendHelp = redirect.includes('#');
  if (runModeIsProd && !needBackendHelp) {
    window.location.href = redirect;
    return;
  }

  // use backend to do redirection, which can bypass the browser's limitations of "location"
  const form = createElementFromHTML<HTMLFormElement>(html`<form method="post"></form>`);
  form.action = `${appSubUrl}/-/fetch-redirect?redirect=${encodeURIComponent(redirect)}`;
  document.body.append(form);
  form.submit();
}

function toggleLoadingIndicator(el: HTMLElement, opt: FetchActionOpts, isLoading: boolean) {
  const loadingIndicatorElems = opt.loadingIndicator ? execPseudoSelectorCommands(el, opt.loadingIndicator).targets : [];
  for (const indicatorEl of loadingIndicatorElems) {
    if (isLoading) {
      // for button or input element, we can directly disable it, it looks better than adding a loading spinner
      if ('disabled' in indicatorEl) {
        indicatorEl.disabled = true;
      } else {
        indicatorEl.classList.add('is-loading');
        if (indicatorEl.clientHeight < 50) indicatorEl.classList.add('loading-icon-2px');
      }
    } else {
      if ('disabled' in indicatorEl) {
        indicatorEl.disabled = false;
      } else {
        indicatorEl.classList.remove('is-loading', 'loading-icon-2px');
      }
    }
  }
}

async function handleFetchActionSuccessJson(el: HTMLElement, respJson: any) {
  ignoreAreYouSure(el); // ignore the areYouSure check before reloading
  if (typeof respJson?.redirect === 'string') {
    fetchActionDoRedirect(respJson.redirect);
  } else {
    window.location.reload();
  }
}

async function handleFetchActionSuccess(el: HTMLElement, opt: FetchActionOpts, resp: Response) {
  const isRespJson = resp.headers.get('content-type')?.includes('application/json');
  const respText = await resp.text();
  const respJson = isRespJson ? JSON.parse(respText) : null;
  if (isRespJson) {
    await handleFetchActionSuccessJson(el, respJson);
  } else if (opt.successSync) {
    await handleFetchActionSuccessSync(el, opt.successSync, respText);
  } else {
    showErrorToast(`Unsupported fetch action response, expected JSON but got: ${respText.substring(0, 200)}`);
  }
}

async function handleFetchActionError(resp: Response) {
  const isRespJson = resp.headers.get('content-type')?.includes('application/json');
  const respText = await resp.text();
  const respJson = isRespJson ? JSON.parse(await resp.text()) : null;
  if (respJson?.errorMessage) {
    // the code was quite messy, sometimes the backend uses "err", sometimes it uses "error", and even "user_error"
    // but at the moment, as a new approach, we only use "errorMessage" here, backend can use JSONError() to respond.
    showErrorToast(respJson.errorMessage, {useHtmlBody: respJson.renderFormat === 'html'});
  } else {
    showErrorToast(`Error ${resp.status} ${resp.statusText}. Response: ${respText.substring(0, 200)}`);
  }
}

function buildFetchActionUrl(el: HTMLElement, opt: FetchActionOpts) {
  let url = opt.url;
  if ('name' in el && 'value' in el) {
    // ref: https://htmx.org/attributes/hx-get/
    // If the element with the hx-get attribute also has a value, this will be included as a parameter
    const name = (el as HTMLInputElement).name;
    const val = (el as HTMLInputElement).value;
    const u = new URL(url, window.location.href);
    if (name && !u.searchParams.has(name)) {
      u.searchParams.set(name, val);
      url = u.toString();
    }
  }
  return url;
}

async function performActionRequest(el: HTMLElement, opt: FetchActionOpts) {
  const attrIsLoading = 'data-fetch-is-loading';
  if (el.getAttribute(attrIsLoading)) return;
  if (!await confirmFetchAction(el)) return;

  el.setAttribute(attrIsLoading, 'true');
  toggleLoadingIndicator(el, opt, true);

  try {
    const url = buildFetchActionUrl(el, opt);
    const headers = new Headers(opt.headers);
    headers.set('X-Gitea-Fetch-Action', '1');
    const resp = await request(url, {method: opt.method, body: opt.body, headers});
    if (resp.ok) {
      await handleFetchActionSuccess(el, opt, resp);
      return;
    }
    await handleFetchActionError(resp);
  } catch (e) {
    if (e.name !== 'AbortError') {
      console.error(`Fetch action request error:`, e);
      showErrorToast(`Error: ${e.message ?? e}`);
    }
  } finally {
    toggleLoadingIndicator(el, opt, false);
    el.removeAttribute(attrIsLoading);
  }
}

type SubmitFormFetchActionOpts = {
  formSubmitter?: HTMLElement;
  formData?: FormData;
};

function prepareFormFetchActionOpts(formEl: HTMLFormElement, opts: SubmitFormFetchActionOpts = {}): FetchActionOpts {
  const formMethodUpper = formEl.getAttribute('method')?.toUpperCase() || 'GET';
  const formActionUrl = formEl.getAttribute('action') || window.location.href;
  const formData = opts.formData ?? new FormData(formEl);
  const [submitterName, submitterValue] = [opts.formSubmitter?.getAttribute('name'), opts.formSubmitter?.getAttribute('value')];
  if (submitterName) {
    formData.append(submitterName, submitterValue || '');
  }

  let reqUrl = formActionUrl;
  let reqBody: FormData | undefined;
  if (formMethodUpper === 'GET') {
    const params = new URLSearchParams();
    for (const [key, value] of formData) {
      params.append(key, value as string);
    }
    const pos = reqUrl.indexOf('?');
    if (pos !== -1) {
      reqUrl = reqUrl.slice(0, pos);
    }
    reqUrl += `?${params.toString()}`;
  } else {
    reqBody = formData;
  }
  return {
    method: formMethodUpper,
    url: reqUrl,
    body: reqBody,
    loadingIndicator: '$this', // for form submit, by default, the loading indicator is the whole form
    successSync: formEl.getAttribute('data-fetch-sync') ?? '', // by default, no fetch sync for form submit
  };
}

export async function submitFormFetchAction(formEl: HTMLFormElement, opts: SubmitFormFetchActionOpts = {}) {
  hideToastsAll();
  await performActionRequest(formEl, prepareFormFetchActionOpts(formEl, opts));
}

async function confirmFetchAction(el: HTMLElement) {
  let elModal: HTMLElement | null = null;
  const dataModalConfirm = el.getAttribute('data-modal-confirm') || '';
  if (dataModalConfirm.startsWith('#')) {
    // eslint-disable-next-line unicorn/prefer-query-selector
    elModal = document.getElementById(dataModalConfirm.substring(1));
    if (elModal) {
      elModal = createElementFromHTML(elModal.outerHTML);
      elModal.removeAttribute('id');
    }
  }
  if (!elModal) {
    const modalConfirmContent = dataModalConfirm || el.getAttribute('data-modal-confirm-content') || '';
    if (modalConfirmContent) {
      const isRisky = el.classList.contains('red') || el.classList.contains('negative');
      elModal = createConfirmModal({
        header: el.getAttribute('data-modal-confirm-header') || '',
        content: modalConfirmContent,
        confirmButtonColor: isRisky ? 'red' : 'primary',
      });
    }
  }
  if (!elModal) return true;
  return await confirmModal(elModal);
}

async function performLinkFetchAction(el: HTMLElement) {
  hideToastsAll();
  await performActionRequest(el, {
    method: el.getAttribute('data-fetch-method') || 'POST', // by default, the method is POST for link-action
    url: el.getAttribute('data-url')!,
    loadingIndicator: el.getAttribute('data-fetch-indicator') ?? '$this', // by default, the link-action itself is the loading indicator
    successSync: el.getAttribute('data-fetch-sync') ?? '', // by default, no fetch sync for link-action
  });
}

type FetchActionTriggerType = 'click' | 'change' | 'every' | 'load' | 'fetch-reload';

export async function performFetchActionTrigger(el: HTMLElement, triggerType: FetchActionTriggerType) {
  const isUserInitiated = triggerType === 'click' || triggerType === 'change';
  // for user initiated action, by default, the loading indicator is the element itself, otherwise no loading indicator
  const defaultLoadingIndicator = isUserInitiated ? '$this' : '';

  if (isUserInitiated) hideToastsAll();
  await performActionRequest(el, {
    method: el.getAttribute('data-fetch-method') || 'GET', // by default, the method is GET for fetch trigger action
    url: el.getAttribute('data-fetch-url')!,
    loadingIndicator: el.getAttribute('data-fetch-indicator') ?? defaultLoadingIndicator,
    successSync: el.getAttribute('data-fetch-sync') ?? '$this', // by default, the response will replace the current element
  });
}

type PseudoSelectorCommandResult = {
  targets: Element[];
  cmdInnerHTML: boolean;
  cmdMorph: boolean;
};

export function execPseudoSelectorCommands(el: Element, fullCommand: string): PseudoSelectorCommandResult {
  const cmds = fullCommand.split(' ').map((s) => s.trim()).filter(Boolean) || [];
  let targets = [el], cmdInnerHTML = false, cmdMorph = false;
  for (const cmd of cmds) {
    if (cmd === '$this') {
      targets = [el];
    } else if (cmd === '$body') {
      targets = [document.body];
    } else if (cmd === '$innerHTML') {
      cmdInnerHTML = true;
    } else if (cmd === '$morph') {
      cmdMorph = true;
    } else if (cmd.startsWith('$closest(') && cmd.endsWith(')')) {
      const selector = cmd.substring('$closest('.length, cmd.length - 1);
      const newTargets: Element[] = [];
      for (const target of targets) {
        const closest = target.closest(selector);
        if (closest) newTargets.push(closest);
      }
      targets = newTargets;
    } else {
      const newTargets: Element[] = [];
      for (const target of targets) {
        newTargets.push(...target.querySelectorAll(cmd));
      }
      targets = newTargets;
    }
  }
  return {targets, cmdInnerHTML, cmdMorph};
}

async function handleFetchActionSuccessSync(el: Element, successSync: string, respText: string) {
  const res = execPseudoSelectorCommands(el, successSync);
  if (!res.targets.length) throw new Error(`Fetch-sync command "${successSync}" did not find any target element to update`);
  if (res.targets.length > 1) throw new Error(`Fetch-sync command "${successSync}" found multiple target elements, which is not supported`);
  const target = res.targets[0];
  if (res.cmdMorph) {
    Idiomorph.morph(target, respText, {morphStyle: res.cmdInnerHTML ? 'innerHTML' : 'outerHTML'});
  } else if (res.cmdInnerHTML) {
    target.innerHTML = respText;
  } else {
    target.outerHTML = respText;
  }
  await fetchActionReloadOutdatedElements();
}

async function fetchActionReloadOutdatedElements() {
  const outdatedElems: HTMLElement[] = [];
  for (const outdated of document.querySelectorAll<HTMLElement>('[data-fetch-trigger~="fetch-reload"]')) {
    if (!outdated.id) throw new Error(`Elements with "fetch-reload" trigger must have an id to be reloaded after fetch sync: ${outdated.outerHTML.substring(0, 100)}`);
    outdatedElems.push(outdated);
  }
  if (!outdatedElems.length) return;

  const resp = await GET(window.location.href);
  if (!resp.ok) {
    showErrorToast(`Failed to reload page content after fetch action: ${resp.status} ${resp.statusText}`);
    return;
  }
  const newPageHtml = await resp.text();
  const newPageDom = parseDom(newPageHtml, 'text/html');
  for (const oldEl of outdatedElems) {
    // eslint-disable-next-line unicorn/prefer-query-selector
    const newEl = newPageDom.getElementById(oldEl.id);
    if (newEl) {
      oldEl.replaceWith(newEl);
    } else {
      oldEl.remove();
    }
  }
}

function initFetchActionTriggerEvery(el: HTMLElement, trigger: string) {
  const interval = trigger.substring('every '.length);
  const match = /^(\d+)(ms|s)$/.exec(interval);
  if (!match) throw new Error(`Invalid interval format: ${interval}`);

  const num = parseInt(match[1], 10), unit = match[2];
  const intervalMs = unit === 's' ? num * 1000 : num;
  const fn = async () => {
    try {
      await performFetchActionTrigger(el, 'every');
    } finally {
      // only continue if the element is still in the document
      if (document.contains(el)) {
        setTimeout(fn, intervalMs);
      }
    }
  };
  setTimeout(fn, intervalMs);
}

function initFetchActionTrigger(el: HTMLElement) {
  const trigger = el.getAttribute('data-fetch-trigger');

  // this trigger is managed internally, only triggered after fetch sync success, not triggered by event or timer
  if (trigger === 'fetch-reload') return;

  if (trigger === 'load') {
    performFetchActionTrigger(el, trigger);
  } else if (trigger === 'change') {
    el.addEventListener('change', () => performFetchActionTrigger(el, trigger));
  } else if (trigger?.startsWith('every ')) {
    initFetchActionTriggerEvery(el, trigger);
  } else if (!trigger || trigger === 'click') {
    el.addEventListener('click', (e) => {
      e.preventDefault();
      performFetchActionTrigger(el, 'click');
    });
  } else {
    throw new Error(`Unsupported fetch trigger: ${trigger}`);
  }
}

export function initGlobalFetchAction() {
  // The "fetch-action" framework is a general approach for elements to trigger fetch requests:
  // show confirm dialog (if any), show loading indicators, send fetch request, and redirect or update UI after success.
  //
  // If you need more fine-grained control more details, sometimes it's clearer to write the logic in JavaScript, instead of using this generic framework.
  //
  // Attributes:
  //
  // * data-fetch-method: the HTTP method to use
  //   * default to "GET" for "data-fetch-url" actions, "POST" for "link-action" elements
  //   * this attribute is ignored, the method will be determined by the form's "method" attribute, and default to "GET"
  //
  // * data-fetch-url: the URL for the request
  //
  // * data-fetch-trigger: the event to trigger the fetch action, can be:
  //   * "click", "change" (user-initiated events)
  //   * "load" (triggered on page load)
  //   * "every 5s" (also support "ms" unit)
  //   * "fetch-reload" (only triggered by fetch sync success to reload outdated content)
  //
  // * data-fetch-indicator: the loading indicator element selector, it uses the same syntax as "data-fetch-sync" to find the element(s)
  //
  // * data-fetch-sync: when the response is text (html), the pseudo selectors/commands defined in "data-fetch-sync"
  //   will be used to update the content in the current page. It only supports some simple syntaxes that we need.
  //   "$" prefix means it is our private command (for special logic), the selectors are run one by one from current element.
  //   * "$this": replace the current element with the response
  //   * "$innerHTML": replace innerHTML of the current element with the response, instead of replacing the whole element (outerHTML)
  //   * "$morph": use morph algorithm to update the target element
  //   * "$body #the-id .the-class": query the selector one by one from body
  //   * "$closest(tr) td": pseudo command can help to find the target element in a more flexible way
  //
  // * data-modal-confirm: a "confirm modal dialog" will be shown before taking action.
  //   * it can be a string for the content of the modal dialog
  //   * it has "-header" and "-content" variants to set the header and content of the "confirm modal"
  //   * it can refer an existing modal element by "#the-modal-id"

  addDelegatedEventListener(document, 'submit', '.form-fetch-action', async (el: HTMLFormElement, e) => {
    // "fetch-action" will use the form's data to send the request
    e.preventDefault();
    await submitFormFetchAction(el, {formSubmitter: submitEventSubmitter(e)});
  });

  addDelegatedEventListener(document, 'click', '.link-action', async (el, e) => {
    // `<a class="link-action" data-url="...">` is a shorthand for
    // `<a data-fetch-trigger="click" data-fetch-method="post" data-fetch-url="..." data-fetch-indicator="$this">`
    e.preventDefault();
    await performLinkFetchAction(el);
  });

  registerGlobalSelectorFunc('[data-fetch-url]', initFetchActionTrigger);
}
