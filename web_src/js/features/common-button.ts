import $ from 'jquery';
import {POST} from '../modules/fetch.ts';
import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
import {showErrorToast} from '../modules/toast.ts';

export function initGlobalButtonClickOnEnter() {
  $(document).on('keypress', 'div.ui.button,span.ui.button', (e) => {
    if (e.code === ' ' || e.code === 'Enter') {
      $(e.target).trigger('click');
      e.preventDefault();
    }
  });
}

export function initGlobalDeleteButton() {
  // ".delete-button" shows a confirmation modal defined by `data-modal-id` attribute.
  // Some model/form elements will be filled by `data-id` / `data-name` / `data-data-xxx` attributes.
  // If there is a form defined by `data-form`, then the form will be submitted as-is (without any modification).
  // If there is no form, then the data will be posted to `data-url`.
  // TODO: it's not encouraged to use this method. `show-modal` does far better than this.
  for (const btn of document.querySelectorAll('.delete-button')) {
    btn.addEventListener('click', (e) => {
      e.preventDefault();

      // eslint-disable-next-line github/no-dataset -- code depends on the camel-casing
      const dataObj = btn.dataset;

      const modalId = btn.getAttribute('data-modal-id');
      const modal = document.querySelector(`.delete.modal${modalId ? `#${modalId}` : ''}`);

      // set the modal "display name" by `data-name`
      const modalNameEl = modal.querySelector('.name');
      if (modalNameEl) modalNameEl.textContent = btn.getAttribute('data-name');

      // fill the modal elements with data-xxx attributes: `data-data-organization-name="..."` => `<span class="dataOrganizationName">...</span>`
      for (const [key, value] of Object.entries(dataObj)) {
        if (key.startsWith('data')) {
          const textEl = modal.querySelector(`.${key}`);
          if (textEl) textEl.textContent = value;
        }
      }

      $(modal).modal({
        closable: false,
        onApprove: async () => {
          // if `data-type="form"` exists, then submit the form by the selector provided by `data-form="..."`
          if (btn.getAttribute('data-type') === 'form') {
            const formSelector = btn.getAttribute('data-form');
            const form = document.querySelector(formSelector);
            if (!form) throw new Error(`no form named ${formSelector} found`);
            form.submit();
          }

          // prepare an AJAX form by data attributes
          const postData = new FormData();
          for (const [key, value] of Object.entries(dataObj)) {
            if (key.startsWith('data')) { // for data-data-xxx (HTML) -> dataXxx (form)
              postData.append(key.slice(4), value);
            }
            if (key === 'id') { // for data-id="..."
              postData.append('id', value);
            }
          }

          const response = await POST(btn.getAttribute('data-url'), {data: postData});
          if (response.ok) {
            const data = await response.json();
            window.location.href = data.redirect;
          }
        },
      }).modal('show');
    });
  }
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
}

export function initGlobalShowModal() {
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
        $attrTarget[0].textContent = attrib.value; // FIXME: it should be more strict here, only handle div/span/p
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
