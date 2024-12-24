import {request} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {addDelegatedEventListener, submitEventSubmitter} from '../utils/dom.ts';
import {confirmModal} from './comp/ConfirmModal.ts';
import type {RequestOpts} from '../types.ts';
import {ignoreAreYouSure} from '../vendor/jquery.are-you-sure.ts';

const {appSubUrl, i18n} = window.config;

// fetchActionDoRedirect does real redirection to bypass the browser's limitations of "location"
// more details are in the backend's fetch-redirect handler
function fetchActionDoRedirect(redirect: string) {
  const form = document.createElement('form');
  const input = document.createElement('input');
  form.method = 'post';
  form.action = `${appSubUrl}/-/fetch-redirect`;
  input.type = 'hidden';
  input.name = 'redirect';
  input.value = redirect;
  form.append(input);
  document.body.append(form);
  form.submit();
}

async function fetchActionDoRequest(actionElem: HTMLElement, url: string, opt: RequestOpts) {
  try {
    const resp = await request(url, opt);
    if (resp.status === 200) {
      let {redirect} = await resp.json();
      redirect = redirect || actionElem.getAttribute('data-redirect');
      ignoreAreYouSure(actionElem); // ignore the areYouSure check before reloading
      if (redirect) {
        fetchActionDoRedirect(redirect);
      } else {
        window.location.reload();
      }
      return;
    } else if (resp.status >= 400 && resp.status < 500) {
      const data = await resp.json();
      // the code was quite messy, sometimes the backend uses "err", sometimes it uses "error", and even "user_error"
      // but at the moment, as a new approach, we only use "errorMessage" here, backend can use JSONError() to respond.
      if (data.errorMessage) {
        showErrorToast(data.errorMessage, {useHtmlBody: data.renderFormat === 'html'});
      } else {
        showErrorToast(`server error: ${resp.status}`);
      }
    } else {
      showErrorToast(`server error: ${resp.status}`);
    }
  } catch (e) {
    if (e.name !== 'AbortError') {
      console.error('error when doRequest', e);
      showErrorToast(`${i18n.network_error} ${e}`);
    }
  }
  actionElem.classList.remove('is-loading', 'loading-icon-2px');
}

async function formFetchAction(formEl: HTMLFormElement, e: SubmitEvent) {
  e.preventDefault();
  if (formEl.classList.contains('is-loading')) return;

  formEl.classList.add('is-loading');
  if (formEl.clientHeight < 50) {
    formEl.classList.add('loading-icon-2px');
  }

  const formMethod = formEl.getAttribute('method') || 'get';
  const formActionUrl = formEl.getAttribute('action');
  const formData = new FormData(formEl);
  const formSubmitter = submitEventSubmitter(e);
  const [submitterName, submitterValue] = [formSubmitter?.getAttribute('name'), formSubmitter?.getAttribute('value')];
  if (submitterName) {
    formData.append(submitterName, submitterValue || '');
  }

  let reqUrl = formActionUrl;
  const reqOpt = {method: formMethod.toUpperCase(), body: null};
  if (formMethod.toLowerCase() === 'get') {
    const params = new URLSearchParams();
    for (const [key, value] of formData) {
      params.append(key, value.toString());
    }
    const pos = reqUrl.indexOf('?');
    if (pos !== -1) {
      reqUrl = reqUrl.slice(0, pos);
    }
    reqUrl += `?${params.toString()}`;
  } else {
    reqOpt.body = formData;
  }

  await fetchActionDoRequest(formEl, reqUrl, reqOpt);
}

async function linkAction(el: HTMLElement, e: Event) {
  // A "link-action" can post AJAX request to its "data-url"
  // Then the browser is redirected to: the "redirect" in response, or "data-redirect" attribute, or current URL by reloading.
  // If the "link-action" has "data-modal-confirm" attribute, a confirm modal dialog will be shown before taking action.
  e.preventDefault();
  const url = el.getAttribute('data-url');
  const doRequest = async () => {
    if ('disabled' in el) el.disabled = true; // el could be A or BUTTON, but A doesn't have disabled attribute
    await fetchActionDoRequest(el, url, {method: el.getAttribute('data-link-action-method') || 'POST'});
    if ('disabled' in el) el.disabled = false;
  };

  const modalConfirmContent = el.getAttribute('data-modal-confirm') ||
    el.getAttribute('data-modal-confirm-content') || '';
  if (!modalConfirmContent) {
    await doRequest();
    return;
  }

  const isRisky = el.classList.contains('red') || el.classList.contains('negative');
  if (await confirmModal({
    header: el.getAttribute('data-modal-confirm-header') || '',
    content: modalConfirmContent,
    confirmButtonColor: isRisky ? 'red' : 'primary',
  })) {
    await doRequest();
  }
}

export function initGlobalFetchAction() {
  addDelegatedEventListener(document, 'submit', '.form-fetch-action', formFetchAction);
  addDelegatedEventListener(document, 'click', '.link-action', linkAction);
}
