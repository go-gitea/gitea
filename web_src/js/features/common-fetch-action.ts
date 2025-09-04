import {request} from '../modules/fetch.ts';
import {hideToastsAll, showErrorToast} from '../modules/toast.ts';
import {addDelegatedEventListener, createElementFromHTML, submitEventSubmitter} from '../utils/dom.ts';
import {confirmModal, createConfirmModal} from './comp/ConfirmModal.ts';
import type {RequestOpts} from '../types.ts';
import {ignoreAreYouSure} from '../vendor/jquery.are-you-sure.ts';

const {appSubUrl} = window.config;

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
  const showErrorForResponse = (code: number, message: string) => {
    showErrorToast(`Error ${code || 'request'}: ${message}`);
  };

  let respStatus = 0;
  let respText = '';
  try {
    hideToastsAll();
    const resp = await request(url, opt);
    respStatus = resp.status;
    respText = await resp.text();
    const respJson = JSON.parse(respText);
    if (respStatus === 200) {
      let {redirect} = respJson;
      redirect = redirect || actionElem.getAttribute('data-redirect');
      ignoreAreYouSure(actionElem); // ignore the areYouSure check before reloading
      if (redirect) {
        fetchActionDoRedirect(redirect);
      } else {
        window.location.reload();
      }
      return;
    }

    if (respStatus >= 400 && respStatus < 500 && respJson?.errorMessage) {
      // the code was quite messy, sometimes the backend uses "err", sometimes it uses "error", and even "user_error"
      // but at the moment, as a new approach, we only use "errorMessage" here, backend can use JSONError() to respond.
      showErrorToast(respJson.errorMessage, {useHtmlBody: respJson.renderFormat === 'html'});
    } else {
      showErrorForResponse(respStatus, respText);
    }
  } catch (e) {
    if (e.name === 'SyntaxError') {
      showErrorForResponse(respStatus, (respText || '').substring(0, 100));
    } else if (e.name !== 'AbortError') {
      console.error('fetchActionDoRequest error', e);
      showErrorForResponse(respStatus, `${e}`);
    }
  }
  actionElem.classList.remove('is-loading', 'loading-icon-2px');
}

async function onFormFetchActionSubmit(formEl: HTMLFormElement, e: SubmitEvent) {
  e.preventDefault();
  await submitFormFetchAction(formEl, submitEventSubmitter(e));
}

export async function submitFormFetchAction(formEl: HTMLFormElement, formSubmitter?: HTMLElement) {
  if (formEl.classList.contains('is-loading')) return;

  formEl.classList.add('is-loading');
  if (formEl.clientHeight < 50) {
    formEl.classList.add('loading-icon-2px');
  }

  const formMethod = formEl.getAttribute('method') || 'get';
  const formActionUrl = formEl.getAttribute('action') || window.location.href;
  const formData = new FormData(formEl);
  const [submitterName, submitterValue] = [formSubmitter?.getAttribute('name'), formSubmitter?.getAttribute('value')];
  if (submitterName) {
    formData.append(submitterName, submitterValue || '');
  }

  let reqUrl = formActionUrl;
  const reqOpt = {
    method: formMethod.toUpperCase(),
    body: null as FormData | null,
  };
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

async function onLinkActionClick(el: HTMLElement, e: Event) {
  // A "link-action" can post AJAX request to its "data-url"
  // Then the browser is redirected to: the "redirect" in response, or "data-redirect" attribute, or current URL by reloading.
  // If the "link-action" has "data-modal-confirm" attribute, a "confirm modal dialog" will be shown before taking action.
  // Attribute "data-modal-confirm" can be a modal element by "#the-modal-id", or a string content for the modal dialog.
  e.preventDefault();
  const url = el.getAttribute('data-url');
  const doRequest = async () => {
    if ('disabled' in el) el.disabled = true; // el could be A or BUTTON, but "A" doesn't have the "disabled" attribute
    await fetchActionDoRequest(el, url, {method: el.getAttribute('data-link-action-method') || 'POST'});
    if ('disabled' in el) el.disabled = false;
  };

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

  if (!elModal) {
    await doRequest();
    return;
  }

  if (await confirmModal(elModal)) {
    await doRequest();
  }
}

export function initGlobalFetchAction() {
  addDelegatedEventListener(document, 'submit', '.form-fetch-action', onFormFetchActionSubmit);
  addDelegatedEventListener(document, 'click', '.link-action', onLinkActionClick);
}
