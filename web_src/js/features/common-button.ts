import {POST} from '../modules/fetch.ts';
import {addDelegatedEventListener, hideElem, isElemVisible, showElem, toggleElem} from '../utils/dom.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {camelize} from 'vue';

export function initGlobalButtonClickOnEnter(): void {
  addDelegatedEventListener(document, 'keypress', 'div.ui.button, span.ui.button', (el, e: KeyboardEvent) => {
    if (e.code === 'Space' || e.code === 'Enter') {
      e.preventDefault();
      el.click();
    }
  });
}

export function initGlobalDeleteButton(): void {
  // ".delete-button" shows a confirmation modal defined by `data-modal-id` attribute.
  // Some model/form elements will be filled by `data-id` / `data-name` / `data-data-xxx` attributes.
  // If there is a form defined by `data-form`, then the form will be submitted as-is (without any modification).
  // If there is no form, then the data will be posted to `data-url`.
  // TODO: do not use this method in new code. `show-modal` / `link-action(data-modal-confirm)` does far better than this.
  // FIXME: all legacy `delete-button` should be refactored to use `show-modal` or `link-action`
  for (const btn of document.querySelectorAll<HTMLElement>('.delete-button')) {
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

      fomanticQuery(modal).modal({
        closable: false,
        onApprove: () => {
          // if `data-type="form"` exists, then submit the form by the selector provided by `data-form="..."`
          if (btn.getAttribute('data-type') === 'form') {
            const formSelector = btn.getAttribute('data-form');
            const form = document.querySelector<HTMLFormElement>(formSelector);
            if (!form) throw new Error(`no form named ${formSelector} found`);
            modal.classList.add('is-loading'); // the form is not in the modal, so also add loading indicator to the modal
            form.classList.add('is-loading');
            form.submit();
            return false; // prevent modal from closing automatically
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
          (async () => {
            const response = await POST(btn.getAttribute('data-url'), {data: postData});
            if (response.ok) {
              const data = await response.json();
              window.location.href = data.redirect;
            }
          })();
          modal.classList.add('is-loading'); // the request is in progress, so also add loading indicator to the modal
          return false; // prevent modal from closing automatically
        },
      }).modal('show');
    });
  }
}

function onShowPanelClick(el: HTMLElement, e: MouseEvent) {
  // a '.show-panel' element can show a panel, by `data-panel="selector"`
  // if it has "toggle" class, it toggles the panel
  e.preventDefault();
  const sel = el.getAttribute('data-panel');
  const elems = el.classList.contains('toggle') ? toggleElem(sel) : showElem(sel);
  for (const elem of elems) {
    if (isElemVisible(elem as HTMLElement)) {
      elem.querySelector<HTMLElement>('[autofocus]')?.focus();
    }
  }
}

function onHidePanelClick(el: HTMLElement, e: MouseEvent) {
  // a `.hide-panel` element can hide a panel, by `data-panel="selector"` or `data-panel-closest="selector"`
  e.preventDefault();
  let sel = el.getAttribute('data-panel');
  if (sel) {
    hideElem(sel);
    return;
  }
  sel = el.getAttribute('data-panel-closest');
  if (sel) {
    hideElem((el.parentNode as HTMLElement).closest(sel));
    return;
  }
  throw new Error('no panel to hide'); // should never happen, otherwise there is a bug in code
}

export function assignElementProperty(el: any, name: string, val: string) {
  name = camelize(name);
  const old = el[name];
  if (typeof old === 'boolean') {
    el[name] = val === 'true';
  } else if (typeof old === 'number') {
    el[name] = parseFloat(val);
  } else if (typeof old === 'string') {
    el[name] = val;
  } else {
    // in the future, we could introduce a better typing system like `data-modal-form.action:string="..."`
    throw new Error(`cannot assign element property ${name} by value ${val}`);
  }
}

function onShowModalClick(el: HTMLElement, e: MouseEvent) {
  // A ".show-modal" button will show a modal dialog defined by its "data-modal" attribute.
  // Each "data-modal-{target}" attribute will be filled to target element's value or text-content.
  // * First, try to query '#target'
  // * Then, try to query '[name=target]'
  // * Then, try to query '.target'
  // * Then, try to query 'target' as HTML tag
  // If there is a ".{prop-name}" part like "data-modal-form.action", the "form" element's "action" property will be set, the "prop-name" will be camel-cased to "propName".
  e.preventDefault();
  const modalSelector = el.getAttribute('data-modal');
  const elModal = document.querySelector(modalSelector);
  if (!elModal) throw new Error('no modal for this action');

  const modalAttrPrefix = 'data-modal-';
  for (const attrib of el.attributes) {
    if (!attrib.name.startsWith(modalAttrPrefix)) {
      continue;
    }

    const attrTargetCombo = attrib.name.substring(modalAttrPrefix.length);
    const [attrTargetName, attrTargetProp] = attrTargetCombo.split('.');
    // try to find target by: "#target" -> "[name=target]" -> ".target" -> "<target> tag"
    const attrTarget = elModal.querySelector(`#${attrTargetName}`) ||
      elModal.querySelector(`[name=${attrTargetName}]`) ||
      elModal.querySelector(`.${attrTargetName}`) ||
      elModal.querySelector(`${attrTargetName}`);
    if (!attrTarget) {
      if (!window.config.runModeIsProd) throw new Error(`attr target "${attrTargetCombo}" not found for modal`);
      continue;
    }

    if (attrTargetProp) {
      assignElementProperty(attrTarget, attrTargetProp, attrib.value);
    } else if (attrTarget.matches('input, textarea')) {
      (attrTarget as HTMLInputElement | HTMLTextAreaElement).value = attrib.value; // FIXME: add more supports like checkbox
    } else {
      attrTarget.textContent = attrib.value; // FIXME: it should be more strict here, only handle div/span/p
    }
  }

  fomanticQuery(elModal).modal('show');
}

export function initGlobalButtons(): void {
  // There are many "cancel button" elements in modal dialogs, Fomantic UI expects they are button-like elements but never submit a form.
  // However, Gitea misuses the modal dialog and put the cancel buttons inside forms, so we must prevent the form submission.
  // There are a few cancel buttons in non-modal forms, and there are some dynamically created forms (eg: the "Edit Issue Content")
  addDelegatedEventListener(document, 'click', 'form button.ui.cancel.button', (_ /* el */, e) => e.preventDefault());

  // Ideally these "button" events should be handled by registerGlobalEventFunc
  // Refactoring would involve too many changes, so at the moment, just use the global event listener.
  addDelegatedEventListener(document, 'click', '.show-panel, .hide-panel, .show-modal', (el, e: MouseEvent) => {
    if (el.classList.contains('show-panel')) {
      onShowPanelClick(el, e);
    } else if (el.classList.contains('hide-panel')) {
      onHidePanelClick(el, e);
    } else if (el.classList.contains('show-modal')) {
      onShowModalClick(el, e);
    }
  });
}
