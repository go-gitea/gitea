import {addDelegatedEventListener, hideElem, isElemVisible, showElem, toggleElem} from '../utils/dom.ts';
import {showFomanticModal} from '../modules/fomantic/modal.ts';
import {camelize} from 'vue';
import {applyAutoFocus} from './common-page.ts';

export function initGlobalButtonClickOnEnter(): void {
  addDelegatedEventListener(document, 'keypress', 'div.ui.button, span.ui.button', (el, e: KeyboardEvent) => {
    if (e.code === 'Space' || e.code === 'Enter') {
      e.preventDefault();
      el.click();
    }
  });
}

function onShowPanelClick(el: HTMLElement, e: MouseEvent) {
  // a '.show-panel' element can show a panel, by `data-panel="selector"`
  // if it has "toggle" class, it toggles the panel
  e.preventDefault();
  const sel = el.getAttribute('data-panel')!;
  const elems = el.classList.contains('toggle') ? toggleElem(sel) : showElem(sel);
  for (const elem of elems) {
    if (isElemVisible(elem as HTMLElement)) {
      applyAutoFocus(elem);
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
    hideElem((el.parentNode as HTMLElement).closest(sel)!);
    return;
  }
  throw new Error('no panel to hide'); // should never happen, otherwise there is a bug in code
}

export type ElementWithAssignableProperties = {
  getAttribute: (name: string) => string | null;
  setAttribute: (name: string, value: string) => void;
} & Record<string, any>;

export function assignElementProperty(el: ElementWithAssignableProperties, kebabName: string, val: string) {
  const camelizedName = camelize(kebabName);
  const old = el[camelizedName];
  if (typeof old === 'boolean') {
    el[camelizedName] = val === 'true';
  } else if (typeof old === 'number') {
    el[camelizedName] = parseFloat(val);
  } else if (typeof old === 'string') {
    el[camelizedName] = val;
  } else if (old?.nodeName) {
    // "form" has an edge case: its "<input name=action>" element overwrites the "action" property, we can only set attribute
    el.setAttribute(kebabName, val);
  } else {
    // in the future, we could introduce a better typing system like `data-modal-form.action:string="..."`
    throw new Error(`cannot assign element property "${camelizedName}" by value "${val}"`);
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
  const modalSelector = el.getAttribute('data-modal')!;
  const elModal = document.querySelector(modalSelector);
  if (!elModal) throw new Error('no modal for this action');

  const modalAttrPrefix = 'data-modal-';
  for (const attrib of el.attributes) {
    if (!attrib.name.startsWith(modalAttrPrefix)) {
      continue;
    }

    const attrTargetCombo = attrib.name.substring(modalAttrPrefix.length);
    const [attrTargetName, attrTargetProp] = attrTargetCombo.split('.');
    // try to find target by: "#target" -> "[name=target]" -> ".target" -> "<target> tag", and then try the modal itself
    const attrTarget = elModal.querySelector(`#${attrTargetName}`) ||
      elModal.querySelector(`[name=${CSS.escape(attrTargetName)}]`) ||
      elModal.querySelector(`.${attrTargetName}`) ||
      elModal.querySelector(attrTargetName) ||
      (elModal.matches(attrTargetName) || elModal.matches(`#${attrTargetName}`) || elModal.matches(`.${attrTargetName}`) ? elModal : null);
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

  showFomanticModal(elModal);
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
