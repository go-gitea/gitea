import {registerGlobalInitFunc} from './observer.ts';
import {hideElem, toggleElem} from '../utils/dom.ts';

function initShortcutKbd(kbd: HTMLElement) {
  // Handle initial state: hide the kbd hint if the associated input already has a value
  // (e.g., from browser autofill or back/forward navigation cache)
  const elem = elemFromKbd(kbd);
  if (elem?.value) hideElem(kbd);
  kbd.setAttribute('aria-hidden', 'true');
  kbd.setAttribute('aria-keyshortcuts', kbd.getAttribute('data-shortcut-keys')!);
}

function elemFromKbd(kbd: HTMLElement): HTMLInputElement | HTMLTextAreaElement | null {
  return kbd.parentElement!.querySelector<HTMLInputElement>('input, textarea') || null;
}

function kbdFromElem(input: HTMLElement): HTMLElement | null {
  return input.parentElement!.querySelector<HTMLElement>('kbd') || null;
}

export function initGlobalShortcut() {
  registerGlobalInitFunc('onGlobalShortcut', initShortcutKbd);

  // A <kbd> element next to an <input> declares a keyboard shortcut for that input.
  // When the matching key is pressed, the sibling input is focused.
  // When Escape is pressed inside such an input, the input is cleared and blurred.
  // The <kbd> element is shown/hidden automatically based on input focus and value.
  document.addEventListener('keydown', (e: KeyboardEvent) => {
    // Modifier keys are not supported yet
    if (e.ctrlKey || e.metaKey || e.altKey) return;

    const target = e.target as HTMLElement;

    // Handle Escape: clear and blur inputs that have an associated keyboard shortcut
    if (e.key === 'Escape') {
      const kbd = kbdFromElem(target);
      if (kbd) {
        (target as HTMLInputElement).value = '';
        (target as HTMLInputElement).blur();
      }
      return;
    }

    // Don't trigger shortcuts when typing in input fields or contenteditable areas
    if (target.matches('input, textarea, select') || target.isContentEditable) {
      return;
    }

    // Find kbd element with matching shortcut (case-insensitive), then focus its sibling input
    const key = e.key.toLowerCase();
    // At the moment, only a simple match. In the future, it can be extended to support modifiers and key combinations
    const kbd = document.querySelector<HTMLElement>(`.global-shortcut-wrapper > kbd[data-shortcut-keys="${CSS.escape(key)}"]`);
    if (!kbd) return;
    e.preventDefault();
    elemFromKbd(kbd)!.focus();
  });

  // Toggle kbd shortcut hint visibility on input focus/blur
  document.addEventListener('focusin', (e) => {
    const kbd = kbdFromElem(e.target as HTMLElement);
    if (!kbd) return;
    hideElem(kbd);
  });

  document.addEventListener('focusout', (e) => {
    const kbd = kbdFromElem(e.target as HTMLElement);
    if (!kbd) return;
    const hasContent = Boolean((e.target as HTMLInputElement).value);
    toggleElem(kbd, !hasContent);
  });
}
