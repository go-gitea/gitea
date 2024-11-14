import {applyAreYouSure, initAreYouSure} from '../vendor/jquery.are-you-sure.ts';
import {handleGlobalEnterQuickSubmit} from './comp/QuickSubmit.ts';
import {queryElems} from '../utils/dom.ts';
import {initComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';

export function initGlobalFormDirtyLeaveConfirm() {
  initAreYouSure(window.jQuery);
  // Warn users that try to leave a page after entering data into a form.
  // Except on sign-in pages, and for forms marked as 'ignore-dirty'.
  if (!document.querySelector('.page-content.user.signin')) {
    applyAreYouSure('form:not(.ignore-dirty)');
  }
}

export function initGlobalEnterQuickSubmit() {
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    const target = e.target as HTMLElement;
    const hasCtrlOrMeta = ((e.ctrlKey || e.metaKey) && !e.altKey);
    if (hasCtrlOrMeta && target.matches('textarea')) {
      if (handleGlobalEnterQuickSubmit(e.target)) {
        e.preventDefault();
      }
    } else if (target.matches('input') && !target.closest('form')) {
      // input in a normal form could handle Enter key by default, so we only handle the input outside a form
      // eslint-disable-next-line unicorn/no-lonely-if
      if (handleGlobalEnterQuickSubmit(e.target)) {
        e.preventDefault();
      }
    }
  });
}

export function initGlobalComboMarkdownEditor() {
  queryElems<HTMLElement>(document, '.combo-markdown-editor:not(.custom-init)', (el) => initComboMarkdownEditor(el));
}
