import {registerGlobalInitFunc} from './observer.ts';
import {createElementFromHTML, hideElem, toggleElem} from '../utils/dom.ts';
import {showFomanticModal} from './fomantic/modal.ts';
import {html} from '../utils/html.ts';
import {svgRaw} from '../svg.ts';

type ShortcutHandler = (el: HTMLElement) => boolean;

const shortcutHandlers: Record<string, ShortcutHandler[]> = {};

type ShortcutPress = {
  fullKey: string,
  pressTime: number,
};

const shortcutPresses: ShortcutPress[] = [];

function initShortcutKbd(kbd: HTMLElement) {
  // Handle initial state: hide the kbd hint if the associated input already has a value
  // (e.g., from browser autofill or back/forward navigation cache)
  const elem = elemFromKbd(kbd);
  if (elem?.value) hideElem(kbd);
  kbd.setAttribute('aria-hidden', 'true');
  kbd.setAttribute('aria-keyshortcuts', kbd.getAttribute('data-shortcut-keys')!);
  kbd.addEventListener('click', () => elemFromKbd(kbd)?.focus());
}

function shortcutWrapper(el: HTMLElement): HTMLElement | null {
  const parent = el.parentElement;
  return parent?.matches('.global-shortcut-wrapper') ? parent : null;
}

function elemFromKbd(kbd: HTMLElement): HTMLInputElement | HTMLTextAreaElement | null {
  return shortcutWrapper(kbd)?.querySelector<HTMLInputElement>('input, textarea') || null;
}

function kbdFromElem(input: HTMLElement): HTMLElement | null {
  return shortcutWrapper(input)?.querySelector<HTMLElement>('kbd') || null;
}

export function makeFullKeyFromEvent(e: KeyboardEvent): string {
  // follow the MDN standard: https://developer.mozilla.org/en-US/docs/Web/Accessibility/ARIA/Reference/Attributes/aria-keyshortcuts
  // always use PascalCase (upper case for single letter), and sort the modifiers in the order in alphabetical order
  const modifierKeys = ['Alt', 'Control', 'Meta', 'Shift'];
  let key = e.key;
  if (!key || modifierKeys.includes(key)) return '';
  if (key.length === 1) key = key.toUpperCase();
  if (key === ' ') key = 'Space';
  if (key === '+') key = 'Plus';

  const modifiers: string[] = [];
  for (const mod of modifierKeys) {
    if (e.getModifierState(mod)) modifiers.push(mod);
  }
  return [...modifiers, key].join('+');
}

function getShortcutTextContent(elem: Element): string {
  for (const child of elem.childNodes) {
    let text = '';
    if (child instanceof Text) {
      text = child.data.trim();
    } else if (child instanceof Element) {
      // in the future, we can also exclude elements that should not contribute to shortcut labels.
      text = getShortcutTextContent(child);
    }
    if (text) return text;
  }
  return '';
}

function showShortcutHelp() {
  let modal = document.querySelector<HTMLElement>('#global-shortcut-help');
  if (!modal) {
    modal = createElementFromHTML<HTMLElement>(html`
      <div id="global-shortcut-help" class="ui small modal" role="dialog" aria-modal="true">
        <div class="header">
          Keyboard Shortcuts
          ${svgRaw('octicon-x', 16, 'close-modal')}
        </div>
        <div class="scrolling content">
          <table class="ui very basic compact unstackable table">
            <tbody class="shortcut-list"></tbody>
          </table>
        </div>
      </div>
    `);
    document.body.append(modal);
  }

  const list = modal.querySelector<HTMLElement>('.shortcut-list')!;
  list.replaceChildren(createElementFromHTML(html`<tr><td><kbd>?</kbd></td><td>Keyboard Shortcuts</td></tr>`));
  for (const elem of document.querySelectorAll<HTMLElement>('[data-shortcut-keys]')) {
    const target = elem.matches('kbd') ? elemFromKbd(elem) : elem;

    // TODO: which element should be triggered by shortcut: can be fine-tuned in the future
    if (!target || target.matches(':disabled, .disabled')) continue;

    const label = target.getAttribute('aria-label') ||
      target.getAttribute('placeholder') ||
      target.getAttribute('title') ||
      getShortcutTextContent(target);
    list.append(createElementFromHTML(html`<tr><td><kbd>${elem.getAttribute('data-shortcut-keys')}</kbd></td><td>${label}</td></tr>`));
  }
  showFomanticModal(modal);
}

export function registerShortcutHandler(fullKey: string, handler: ShortcutHandler) {
  fullKey = fullKey.toLowerCase();
  shortcutHandlers[fullKey] ||= [];
  shortcutHandlers[fullKey].push(handler);
}

export function initGlobalShortcut() {
  registerGlobalInitFunc('onGlobalShortcut', initShortcutKbd);

  // A <kbd> element next to an <input> declares a keyboard shortcut for that input.
  // When the matching key is pressed, the sibling input is focused.
  // When Escape is pressed inside such an input, the input is cleared and blurred.
  // The <kbd> element is shown/hidden automatically based on input focus and value.
  document.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.isComposing || e.defaultPrevented) {
      shortcutPresses.length = 0;
      return;
    }
    if (e.repeat) return;

    const isPlainKey = !e.ctrlKey && !e.altKey && !e.metaKey;
    const target = e.target as HTMLElement;

    // Handle Escape: clear and blur inputs that have an associated keyboard shortcut
    if (e.key === 'Escape' && isPlainKey) {
      shortcutPresses.length = 0;
      const kbd = kbdFromElem(target);
      if (kbd) {
        (target as HTMLInputElement).value = '';
        (target as HTMLInputElement).blur();
      }
      return;
    }

    // Don't trigger shortcuts when typing in input fields or contenteditable areas
    let shouldSkip = target.matches('input, textarea, select') || target.isContentEditable;
    // Skip shortcuts when a modal or dropdown is open (active), to avoid interfering with their own keyboard handling
    shouldSkip = shouldSkip || Boolean(document.querySelector('.ui.modal.active') || document.querySelector('.ui.dropdown.active'));
    if (shouldSkip) {
      shortcutPresses.length = 0;
      return;
    }

    // Show the keyboard shortcut help modal when '?' is pressed without modifiers
    if (e.key === '?' && isPlainKey) {
      e.preventDefault();
      shortcutPresses.length = 0;
      showShortcutHelp();
      return;
    }

    const fullKey = makeFullKeyFromEvent(e);
    // we don't support multiple modifier keys as a single shortcut (e.g., press Ctrl twice) at the moment
    if (!fullKey) return;

    // just a simple algorithm to detect multi-key shortcuts, so far so good, for daily usage
    // if any bad case happens, it can be fine-tuned in the future.
    const keyPressDelay = 1000;
    const pressTime = Date.now();
    while (shortcutPresses.length && pressTime - shortcutPresses[0].pressTime > keyPressDelay) {
      shortcutPresses.shift();
    }
    shortcutPresses.push({fullKey, pressTime});

    const fullKeys = shortcutPresses.map((p) => p.fullKey).join(' ');
    for (const handler of shortcutHandlers[fullKeys.toLowerCase()] || []) {
      if (handler(target)) {
        e.preventDefault();
        shortcutPresses.length = 0;
        return;
      }
    }

    // It's said that "spaces in aria-keyshortcuts separate alternative shortcuts", but we want to support multi-key shortcuts
    // So here we use our own data attribute to store the full shortcut keys, and match it case-insensitively.
    const matchedElems = document.querySelectorAll<HTMLElement>(`[data-shortcut-keys="${CSS.escape(fullKeys)}" i]`);
    if (!matchedElems.length) return;
    // TODO: if there are multiple matches, maybe we could introduce some priority rules to determine which one to trigger
    const matchedElem = matchedElems[0];
    e.preventDefault();
    shortcutPresses.length = 0;
    matchedElem.click();
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
