import $ from 'jquery';
import '../vendor/jquery.are-you-sure.js';
import {clippie} from 'clippie';
import {createDropzone} from './dropzone.js';
import {showGlobalErrorMessage} from '../bootstrap.js';
import {handleGlobalEnterQuickSubmit} from './comp/QuickSubmit.js';
import {svg} from '../svg.js';
import {hideElem, showElem, toggleElem, initSubmitEventPolyfill, submitEventSubmitter} from '../utils/dom.js';
import {htmlEscape} from 'escape-goat';
import {showTemporaryTooltip} from '../modules/tippy.js';
import {confirmModal} from './comp/ConfirmModal.js';
import {showErrorToast} from '../modules/toast.js';
import {request, POST, GET} from '../modules/fetch.js';
import '../htmx.js';

const {appUrl, appSubUrl, csrfToken, i18n} = window.config;

export function initGlobalFormDirtyLeaveConfirm() {
  // Warn users that try to leave a page after entering data into a form.
  // Except on sign-in pages, and for forms marked as 'ignore-dirty'.
  if (!$('.user.signin').length) {
    $('form:not(.ignore-dirty)').areYouSure();
  }
}

export function initHeadNavbarContentToggle() {
  const navbar = document.getElementById('navbar');
  const btn = document.getElementById('navbar-expand-toggle');
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

export function initGlobalEnterQuickSubmit() {
  document.addEventListener('keydown', (e) => {
    const isQuickSubmitEnter = ((e.ctrlKey && !e.altKey) || e.metaKey) && (e.key === 'Enter');
    if (isQuickSubmitEnter && e.target.matches('textarea')) {
      e.preventDefault();
      handleGlobalEnterQuickSubmit(e.target);
    }
  });
}

export function initGlobalButtonClickOnEnter() {
  $(document).on('keypress', 'div.ui.button,span.ui.button', (e) => {
    if (e.code === ' ' || e.code === 'Enter') {
      $(e.target).trigger('click');
      e.preventDefault();
    }
  });
}

// fetchActionDoRedirect does real redirection to bypass the browser's limitations of "location"
// more details are in the backend's fetch-redirect handler
function fetchActionDoRedirect(redirect) {
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

async function fetchActionDoRequest(actionElem, url, opt) {
  try {
    const resp = await request(url, opt);
    if (resp.status === 200) {
      let {redirect} = await resp.json();
      redirect = redirect || actionElem.getAttribute('data-redirect');
      actionElem.classList.remove('dirty'); // remove the areYouSure check before reloading
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

async function formFetchAction(e) {
  if (!e.target.classList.contains('form-fetch-action')) return;

  e.preventDefault();
  const formEl = e.target;
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
  const reqOpt = {method: formMethod.toUpperCase()};
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

export function initGlobalCommon() {
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

  $('.tabular.menu .item').tab();

  initSubmitEventPolyfill();
  document.addEventListener('submit', formFetchAction);
  document.addEventListener('click', linkAction);
}

export function initGlobalDropzone() {
  for (const el of document.querySelectorAll('.dropzone')) {
    initDropzone(el);
  }
}

export function initDropzone(el) {
  const $dropzone = $(el);
  const _promise = createDropzone(el, {
    url: $dropzone.data('upload-url'),
    headers: {'X-Csrf-Token': csrfToken},
    maxFiles: $dropzone.data('max-file'),
    maxFilesize: $dropzone.data('max-size'),
    acceptedFiles: (['*/*', ''].includes($dropzone.data('accepts'))) ? null : $dropzone.data('accepts'),
    addRemoveLinks: true,
    dictDefaultMessage: $dropzone.data('default-message'),
    dictInvalidFileType: $dropzone.data('invalid-input-type'),
    dictFileTooBig: $dropzone.data('file-too-big'),
    dictRemoveFile: $dropzone.data('remove-file'),
    timeout: 0,
    thumbnailMethod: 'contain',
    thumbnailWidth: 480,
    thumbnailHeight: 480,
    init() {
      this.on('success', (file, data) => {
        file.uuid = data.uuid;
        const $input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
        $dropzone.find('.files').append($input);
        // Create a "Copy Link" element, to conveniently copy the image
        // or file link as Markdown to the clipboard
        const copyLinkElement = document.createElement('div');
        copyLinkElement.className = 'tw-text-center';
        // The a element has a hardcoded cursor: pointer because the default is overridden by .dropzone
        copyLinkElement.innerHTML = `<a href="#" style="cursor: pointer;">${svg('octicon-copy', 14, 'copy link')} Copy link</a>`;
        copyLinkElement.addEventListener('click', async (e) => {
          e.preventDefault();
          let fileMarkdown = `[${file.name}](/attachments/${file.uuid})`;
          if (file.type.startsWith('image/')) {
            fileMarkdown = `!${fileMarkdown}`;
          } else if (file.type.startsWith('video/')) {
            fileMarkdown = `<video src="/attachments/${file.uuid}" title="${htmlEscape(file.name)}" controls></video>`;
          }
          const success = await clippie(fileMarkdown);
          showTemporaryTooltip(e.target, success ? i18n.copy_success : i18n.copy_error);
        });
        file.previewTemplate.append(copyLinkElement);
      });
      this.on('removedfile', (file) => {
        $(`#${file.uuid}`).remove();
        if ($dropzone.data('remove-url')) {
          POST($dropzone.data('remove-url'), {
            data: new URLSearchParams({file: file.uuid}),
          });
        }
      });
      this.on('error', function (file, message) {
        showErrorToast(message);
        this.removeFile(file);
      });
    },
  });
}

async function linkAction(e) {
  // A "link-action" can post AJAX request to its "data-url"
  // Then the browser is redirected to: the "redirect" in response, or "data-redirect" attribute, or current URL by reloading.
  // If the "link-action" has "data-modal-confirm" attribute, a confirm modal dialog will be shown before taking action.
  const el = e.target.closest('.link-action');
  if (!el) return;

  e.preventDefault();
  const url = el.getAttribute('data-url');
  const doRequest = async () => {
    el.disabled = true;
    await fetchActionDoRequest(el, url, {method: 'POST'});
    el.disabled = false;
  };

  const modalConfirmContent = htmlEscape(el.getAttribute('data-modal-confirm') || '');
  if (!modalConfirmContent) {
    await doRequest();
    return;
  }

  const isRisky = el.classList.contains('red') || el.classList.contains('yellow') || el.classList.contains('orange') || el.classList.contains('negative');
  if (await confirmModal({content: modalConfirmContent, buttonColor: isRisky ? 'orange' : 'primary'})) {
    await doRequest();
  }
}

export function initGlobalLinkActions() {
  function showDeletePopup(e) {
    e.preventDefault();
    const $this = $(this);
    const dataArray = $this.data();
    let filter = '';
    if (this.getAttribute('data-modal-id')) {
      filter += `#${this.getAttribute('data-modal-id')}`;
    }

    const $dialog = $(`.delete.modal${filter}`);
    $dialog.find('.name').text($this.data('name'));
    for (const [key, value] of Object.entries(dataArray)) {
      if (key && key.startsWith('data')) {
        $dialog.find(`.${key}`).text(value);
      }
    }

    $dialog.modal({
      closable: false,
      onApprove: async () => {
        if ($this.data('type') === 'form') {
          $($this.data('form')).trigger('submit');
          return;
        }
        const postData = new FormData();
        for (const [key, value] of Object.entries(dataArray)) {
          if (key && key.startsWith('data')) {
            postData.append(key.slice(4), value);
          }
          if (key === 'id') {
            postData.append('id', value);
          }
        }

        const response = await POST($this.data('url'), {data: postData});
        if (response.ok) {
          const data = await response.json();
          window.location.href = data.redirect;
        }
      },
    }).modal('show');
  }

  // Helpers.
  $('.delete-button').on('click', showDeletePopup);
}

function initGlobalShowModal() {
  // A ".show-modal" button will show a modal dialog defined by its "data-modal" attribute.
  // Each "data-modal-{target}" attribute will be filled to target element's value or text-content.
  // * First, try to query '#target'
  // * Then, try to query '.target'
  // * Then, try to query 'target' as HTML tag
  // If there is a ".{attr}" part like "data-modal-form.action", then the form's "action" attribute will be set.
  $('.show-modal').on('click', function (e) {
    e.preventDefault();
    const modalSelector = this.getAttribute('data-modal');
    const $modal = $(modalSelector);
    if (!$modal.length) {
      throw new Error('no modal for this action');
    }
    const modalAttrPrefix = 'data-modal-';
    for (const attrib of this.attributes) {
      if (!attrib.name.startsWith(modalAttrPrefix)) {
        continue;
      }

      const attrTargetCombo = attrib.name.substring(modalAttrPrefix.length);
      const [attrTargetName, attrTargetAttr] = attrTargetCombo.split('.');
      // try to find target by: "#target" -> ".target" -> "target tag"
      let $attrTarget = $modal.find(`#${attrTargetName}`);
      if (!$attrTarget.length) $attrTarget = $modal.find(`.${attrTargetName}`);
      if (!$attrTarget.length) $attrTarget = $modal.find(`${attrTargetName}`);
      if (!$attrTarget.length) continue; // TODO: show errors in dev mode to remind developers that there is a bug

      if (attrTargetAttr) {
        $attrTarget[0][attrTargetAttr] = attrib.value;
      } else if ($attrTarget[0].matches('input, textarea')) {
        $attrTarget.val(attrib.value); // FIXME: add more supports like checkbox
      } else {
        $attrTarget.text(attrib.value); // FIXME: it should be more strict here, only handle div/span/p
      }
    }

    $modal.modal('setting', {
      onApprove: () => {
        // "form-fetch-action" can handle network errors gracefully,
        // so keep the modal dialog to make users can re-submit the form if anything wrong happens.
        if ($modal.find('.form-fetch-action').length) return false;
      },
    }).modal('show');
  });
}

export function initGlobalButtons() {
  // There are many "cancel button" elements in modal dialogs, Fomantic UI expects they are button-like elements but never submit a form.
  // However, Gitea misuses the modal dialog and put the cancel buttons inside forms, so we must prevent the form submission.
  // There are a few cancel buttons in non-modal forms, and there are some dynamically created forms (eg: the "Edit Issue Content")
  $(document).on('click', 'form button.ui.cancel.button', (e) => {
    e.preventDefault();
  });

  $('.show-panel').on('click', function (e) {
    // a '.show-panel' element can show a panel, by `data-panel="selector"`
    // if it has "toggle" class, it toggles the panel
    e.preventDefault();
    const sel = this.getAttribute('data-panel');
    if (this.classList.contains('toggle')) {
      toggleElem(sel);
    } else {
      showElem(sel);
    }
  });

  $('.hide-panel').on('click', function (e) {
    // a `.hide-panel` element can hide a panel, by `data-panel="selector"` or `data-panel-closest="selector"`
    e.preventDefault();
    let sel = this.getAttribute('data-panel');
    if (sel) {
      hideElem($(sel));
      return;
    }
    sel = this.getAttribute('data-panel-closest');
    if (sel) {
      hideElem($(this).closest(sel));
      return;
    }
    // should never happen, otherwise there is a bug in code
    showErrorToast('Nothing to hide');
  });

  initGlobalShowModal();
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
Mismatched ROOT_URL config causes wrong URL links for web UI/mail content/webhook notification/OAuth2 sign-in.`);
}
