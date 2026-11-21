import {handleGlobalEnterQuickSubmit} from './comp/QuickSubmit.ts';
import {initComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

export function initGlobalEnterQuickSubmit() {
  document.addEventListener('keydown', (e) => {
    if (e.isComposing) return;
    if (e.key !== 'Enter') return;
    const el = e.target as HTMLElement;
    const hasCtrlOrMeta = ((e.ctrlKey || e.metaKey) && !e.altKey);
    const isCtrlEnterInTextarea = hasCtrlOrMeta && el.matches('textarea');
    // an input in a normal form could handle Enter key by default, so we only handle the input outside a form
    const isEnterInBareInput = el.matches('input') && !el.closest('form');
    if ((isCtrlEnterInTextarea || isEnterInBareInput) && handleGlobalEnterQuickSubmit(el)) {
      e.preventDefault();
    }
  });
}

export function initGlobalComboMarkdownEditor() {
  registerGlobalInitFunc('initComboMarkdownEditor', async (el) => {
    await initComboMarkdownEditor(el);
  });
}
