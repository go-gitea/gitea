import $ from 'jquery';
import 'jquery.are-you-sure';
import {createDropzone} from './dropzone.js';
import {initCompColorPicker} from './comp/ColorPicker.js';
import {showGlobalErrorMessage} from '../bootstrap.js';
import {handleGlobalEnterQuickSubmit} from './comp/QuickSubmit.js';
import {svg} from '../svg.js';
import {hideElem, showElem, toggleElem} from '../utils/dom.js';
import {htmlEscape} from 'escape-goat';
import {createTippy} from '../modules/tippy.js';
import {confirmModal} from './comp/ConfirmModal.js';
import {showErrorToast} from '../modules/toast.js';

const {appUrl, appSubUrl, csrfToken, i18n} = window.config;

export function initGlobalFormDirtyLeaveConfirm() {
  // Warn users that try to leave a page after entering data into a form.
  // Except on sign-in pages, and for forms marked as 'ignore-dirty'.
  if ($('.user.signin').length === 0) {
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
  function linkLanguageAction() {
    const $this = $(this);
    $.get($this.data('url')).always(() => {
      window.location.reload();
    });
  }

  $('.language-menu a[lang]').on('click', linkLanguageAction);
}


export function initGlobalEnterQuickSubmit() {
  $(document).on('keydown', '.js-quick-submit', (e) => {
    if (((e.ctrlKey && !e.altKey) || e.metaKey) && (e.key === 'Enter')) {
      handleGlobalEnterQuickSubmit(e.target);
      return false;
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

// doRedirect does real redirection to bypass the browser's limitations of "location"
// more details are in the backend's fetch-redirect handler
function doRedirect(redirect) {
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

async function formFetchAction(e) {
  if (!e.target.classList.contains('form-fetch-action')) return;

  e.preventDefault();
  const formEl = e.target;
  if (formEl.classList.contains('is-loading')) return;

  formEl.classList.add('is-loading');
  if (formEl.clientHeight < 50) {
    formEl.classList.add('small-loading-icon');
  }

  const formMethod = formEl.getAttribute('method') || 'get';
  const formActionUrl = formEl.getAttribute('action');
  const formData = new FormData(formEl);
  const [submitterName, submitterValue] = [e.submitter?.getAttribute('name'), e.submitter?.getAttribute('value')];
  if (submitterName) {
    formData.append(submitterName, submitterValue || '');
  }

  let reqUrl = formActionUrl;
  const reqOpt = {method: formMethod.toUpperCase(), headers: {'X-Csrf-Token': csrfToken}};
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

  let errorTippy;
  const onError = (msg) => {
    formEl.classList.remove('is-loading', 'small-loading-icon');
    if (errorTippy) errorTippy.destroy();
    // TODO: use a better toast UI instead of the tippy. If the form height is large, the tippy position is not good
    errorTippy = createTippy(formEl, {
      content: msg,
      interactive: true,
      showOnCreate: true,
      hideOnClick: true,
      role: 'alert',
      theme: 'form-fetch-error',
      trigger: 'manual',
      arrow: false,
    });
  };

  const doRequest = async () => {
    try {
      const resp = await fetch(reqUrl, reqOpt);
      if (resp.status === 200) {
        const {redirect} = await resp.json();
        formEl.classList.remove('dirty'); // remove the areYouSure check before reloading
        if (redirect) {
          doRedirect(redirect);
        } else {
          window.location.reload();
        }
      } else if (resp.status >= 400 && resp.status < 500) {
        const data = await resp.json();
        // the code was quite messy, sometimes the backend uses "err", sometimes it uses "error", and even "user_error"
        // but at the moment, as a new approach, we only use "errorMessage" here, backend can use JSONError() to respond.
        onError(data.errorMessage || `server error: ${resp.status}`);
      } else {
        onError(`server error: ${resp.status}`);
      }
    } catch (e) {
      console.error('error when doRequest', e);
      onError(i18n.network_error);
    }
  };

  // TODO: add "confirm" support like "link-action" in the future
  await doRequest();
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

  $('.ui.checkbox').checkbox();

  $('.tabular.menu .item').tab();

  document.addEventListener('submit', formFetchAction);
}

export function initGlobalDropzone() {
  // Dropzone
  for (const el of document.querySelectorAll('.dropzone')) {
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
          const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
          $dropzone.find('.files').append(input);
          // Create a "Copy Link" element, to conveniently copy the image
          // or file link as Markdown to the clipboard
          const copyLinkElement = document.createElement('div');
          copyLinkElement.className = 'gt-tc';
          // The a element has a hardcoded cursor: pointer because the default is overridden by .dropzone
          copyLinkElement.innerHTML = `<a href="#" style="cursor: pointer;">${svg('octicon-copy', 14, 'copy link')} Copy link</a>`;
          copyLinkElement.addEventListener('click', (e) => {
            e.preventDefault();
            let fileMarkdown = `[${file.name}](/attachments/${file.uuid})`;
            if (file.type.startsWith('image/')) {
              fileMarkdown = `!${fileMarkdown}`;
            } else if (file.type.startsWith('video/')) {
              fileMarkdown = `<video src="/attachments/${file.uuid}" title="${htmlEscape(file.name)}" controls></video>`;
            }
            navigator.clipboard.writeText(fileMarkdown);
          });
          file.previewTemplate.append(copyLinkElement);
        });
        this.on('removedfile', (file) => {
          $(`#${file.uuid}`).remove();
          if ($dropzone.data('remove-url')) {
            $.post($dropzone.data('remove-url'), {
              file: file.uuid,
              _csrf: csrfToken,
            });
          }
        });
      },
    });
  }
}

async function linkAction(e) {
  e.preventDefault();

  // A "link-action" can post AJAX request to its "data-url"
  // Then the browser is redirected to: the "redirect" in response, or "data-redirect" attribute, or current URL by reloading.
  // If the "link-action" has "data-modal-confirm" attribute, a confirm modal dialog will be shown before taking action.

  const $this = $(this);
  const redirect = $this.attr('data-redirect');

  const doRequest = () => {
    $this.prop('disabled', true);
    $.post($this.attr('data-url'), {
      _csrf: csrfToken
    }).done((data) => {
      if (data && data.redirect) {
        window.location.href = data.redirect;
      } else if (redirect) {
        window.location.href = redirect;
      } else {
        window.location.reload();
      }
    }).always(() => {
      $this.prop('disabled', false);
    });
  };

  const modalConfirmContent = htmlEscape($this.attr('data-modal-confirm') || '');
  if (!modalConfirmContent) {
    doRequest();
    return;
  }

  const isRisky = $this.hasClass('red') || $this.hasClass('yellow') || $this.hasClass('orange') || $this.hasClass('negative');
  if (await confirmModal({content: modalConfirmContent, buttonColor: isRisky ? 'orange' : 'green'})) {
    doRequest();
  }
}

export function initGlobalLinkActions() {
  function showDeletePopup(e) {
    e.preventDefault();
    const $this = $(this);
    const dataArray = $this.data();
    let filter = '';
    if ($this.attr('data-modal-id')) {
      filter += `#${$this.attr('data-modal-id')}`;
    }

    const dialog = $(`.delete.modal${filter}`);
    dialog.find('.name').text($this.data('name'));
    for (const [key, value] of Object.entries(dataArray)) {
      if (key && key.startsWith('data')) {
        dialog.find(`.${key}`).text(value);
      }
    }

    dialog.modal({
      closable: false,
      onApprove() {
        if ($this.data('type') === 'form') {
          $($this.data('form')).trigger('submit');
          return;
        }

        const postData = {
          _csrf: csrfToken,
        };
        for (const [key, value] of Object.entries(dataArray)) {
          if (key && key.startsWith('data')) {
            postData[key.slice(4)] = value;
          }
          if (key === 'id') {
            postData['id'] = value;
          }
        }

        $.post($this.data('url'), postData).done((data) => {
          window.location.href = data.redirect;
        });
      }
    }).modal('show');
  }

  // Helpers.
  $('.delete-button').on('click', showDeletePopup);
  $('.link-action').on('click', linkAction);
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
    const $el = $(this);
    const modalSelector = $el.attr('data-modal');
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
      } else if ($attrTarget.is('input') || $attrTarget.is('textarea')) {
        $attrTarget.val(attrib.value); // FIXME: add more supports like checkbox
      } else {
        $attrTarget.text(attrib.value); // FIXME: it should be more strict here, only handle div/span/p
      }
    }
    const colorPickers = $modal.find('.color-picker');
    if (colorPickers.length > 0) {
      initCompColorPicker(); // FIXME: this might cause duplicate init
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

  $('.show-panel.button').on('click', function (e) {
    // a '.show-panel.button' can show a panel, by `data-panel="selector"`
    // if the button is a "toggle" button, it toggles the panel
    e.preventDefault();
    const sel = $(this).attr('data-panel');
    if (this.classList.contains('toggle')) {
      toggleElem(sel);
    } else {
      showElem(sel);
    }
  });

  $('.hide-panel.button').on('click', function (e) {
    // a `.hide-panel.button` can hide a panel, by `data-panel="selector"` or `data-panel-closest="selector"`
    e.preventDefault();
    let sel = $(this).attr('data-panel');
    if (sel) {
      hideElem($(sel));
      return;
    }
    sel = $(this).attr('data-panel-closest');
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
Mismatched ROOT_URL config causes wrong URL links for web UI/mail content/webhook notification.`);
}
