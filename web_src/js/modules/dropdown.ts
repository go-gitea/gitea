import {createTippy} from './tippy.ts';
import {createElementFromAttrs, generateElemId, isElemVisible, onInputDebounce} from '../utils/dom.ts';
import {debounce} from '../utils/func.ts';
import type {Instance, Placement} from 'tippy.js';

type DropdownKind = 'menu' | 'select' | 'multiselect';

type DropdownOptions = {
  loadItems?: (query: string, signal: AbortSignal) => Promise<Element[]>, // remote items, loaded on every open and while filtering
};

const instances = new WeakMap<Element, Dropdown>(); // keyed by both container and popup
const shownDropdowns = new Set<Dropdown>();

const removalObserver = new MutationObserver(() => {
  for (const dropdown of shownDropdowns) {
    if (!dropdown.container.isConnected) dropdown.hide();
  }
});

const isItemEnabled = (item: Element) => item.getAttribute('aria-disabled') !== 'true';

export class Dropdown {
  container: HTMLElement;
  trigger: HTMLButtonElement;
  popup: HTMLElement;
  valueInput: HTMLInputElement | null;
  filterInput: HTMLInputElement | null;
  kind: DropdownKind;
  delimiter: string;
  loadItems: DropdownOptions['loadItems'];
  tippy: Instance | null = null;
  highlighted: HTMLElement | null = null;
  highlightDelta = 0;
  loadAbort: AbortController | null = null;
  reloadDebounced = onInputDebounce(() => this.reload());
  typeAhead = '';
  resetTypeAhead = debounce(() => { this.typeAhead = '' }, 500);

  constructor(container: HTMLElement, opts: DropdownOptions) {
    this.container = container;
    this.trigger = container.querySelector(':scope > .dropdown-trigger')!;
    this.popup = container.querySelector(':scope > .dropdown-popup')!;
    this.valueInput = container.querySelector(':scope > .dropdown-value');
    this.filterInput = this.popup.querySelector('.dropdown-filter');
    this.kind = (container.getAttribute('data-kind') || 'menu') as DropdownKind;
    this.delimiter = container.getAttribute('data-delimiter') || ',';
    this.loadItems = opts.loadItems;
    instances.set(container, this);
    instances.set(this.popup, this);

    const isMenu = this.kind === 'menu';
    this.popup.id ||= generateElemId('_dropdown_popup_');
    this.popup.tabIndex = -1;
    if (!this.popup.hasAttribute('role')) this.popup.setAttribute('role', isMenu ? 'menu' : 'listbox');
    if (this.kind === 'multiselect') this.popup.setAttribute('aria-multiselectable', 'true');
    this.trigger.setAttribute('aria-haspopup', isMenu ? 'menu' : 'listbox');
    this.trigger.setAttribute('aria-controls', this.popup.id);
    this.trigger.setAttribute('aria-expanded', 'false');
    this.trigger.id ||= generateElemId('_dropdown_trigger_');
    const fieldLabel = container.parentElement!.querySelector(':scope.field > label');
    if (fieldLabel && !this.trigger.hasAttribute('aria-label')) { // like a native select: the field label, then the value
      fieldLabel.id ||= generateElemId('_dropdown_label_');
      this.trigger.setAttribute('aria-labelledby', `${fieldLabel.id} ${this.trigger.id}`);
    }
    this.popup.setAttribute('aria-labelledby', this.trigger.getAttribute('aria-labelledby') ?? this.trigger.id);
    if (this.filterInput) {
      this.filterInput.autocomplete = 'off';
      this.filterInput.setAttribute('aria-controls', this.popup.id);
      if (!isMenu) {
        this.filterInput.setAttribute('role', 'combobox');
        this.filterInput.setAttribute('aria-expanded', 'true');
      }
    }

    this.trigger.addEventListener('click', (e) => {
      if (this.tippy) {
        this.hide();
      } else {
        this.show(e.detail === 0 ? 1 : 0); // zero clicks means Enter or Space
      }
    });
    this.trigger.addEventListener('keydown', (e) => {
      if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
      e.preventDefault();
      this.show(e.key === 'ArrowDown' ? 1 : -1);
    });
    this.popup.addEventListener('keydown', (e) => this.onPopupKeyDown(e));
    this.popup.addEventListener('click', (e) => this.onPopupClick(e), {capture: true}); // close before item handlers run, so they see the popup in its container
    this.popup.addEventListener('pointermove', (e) => {
      const item = (e.target as Element).closest<HTMLElement>('.dropdown-item');
      if (item && item !== this.highlighted && isItemEnabled(item)) this.highlight(item, false);
    });
    this.popup.addEventListener('mousedown', (e) => { // keep the focus on the filter input or the popup
      if (!(e.target as Element).closest('input, textarea, select')) e.preventDefault();
    });
    this.filterInput?.addEventListener('input', () => this.onFilterInput());
    for (const el of [this.container, this.popup]) {
      el.addEventListener('focusout', (e) => {
        if (e.relatedTarget && !this.contains(e.relatedTarget as Node)) this.hide();
      });
    }
  }

  get keyboardOwner(): HTMLElement {
    return this.filterInput ?? this.popup;
  }

  get value(): string {
    return this.valueInput!.value;
  }

  values(): string[] {
    const value = this.value;
    if (this.kind !== 'multiselect') return [value];
    return value ? value.split(this.delimiter) : [];
  }

  isShown(): boolean {
    return Boolean(this.tippy);
  }

  contains(node: Node | null): boolean {
    return this.container.contains(node) || Boolean(this.tippy?.popper.contains(node));
  }

  items(): HTMLElement[] {
    return Array.from(this.popup.querySelectorAll<HTMLElement>('.dropdown-item'));
  }

  navigableItems(items = this.items()): HTMLElement[] {
    return items.filter((item) => isItemEnabled(item) && isElemVisible(item));
  }

  findItem(value: string): HTMLElement | null {
    return this.popup.querySelector<HTMLElement>(`.dropdown-item[data-value="${CSS.escape(value)}"]`);
  }

  show(highlightDelta = 0) {
    if (this.tippy || this.trigger.disabled) return;
    for (const control of this.popup.querySelectorAll('input:not(.dropdown-filter), button, select, textarea')) {
      const form = control.closest('form');
      if (!form || control.hasAttribute('form')) continue;
      form.id ||= generateElemId('_dropdown_form_');
      control.setAttribute('form', form.id);
    }
    this.highlightDelta = highlightDelta;
    this.tippy = createTippy(this.container, {
      content: this.popup,
      theme: 'dropdown',
      role: 'presentation', // the popup carries the menu or listbox role
      aria: {content: null, expanded: false},
      trigger: 'manual',
      interactive: true,
      arrow: false,
      maxWidth: 'none',
      placement: (this.container.getAttribute('data-placement') || 'bottom-start') as Placement,
      appendTo: this.container.closest('.modal-backdrop') ?? document.body, // outside the backdrop, the page is inert
      limitSizeToViewport: {vertical: true},
      onMount: () => this.onMount(),
      onClickOutside: () => this.hide(),
      onHide: () => {
        if (this.popup.contains(document.activeElement)) this.trigger.focus();
      },
      onHidden: (instance) => this.onHidden(instance),
    });
    if (!shownDropdowns.size) removalObserver.observe(document, {subtree: true, childList: true});
    shownDropdowns.add(this);
    this.trigger.setAttribute('aria-expanded', 'true');
    this.trigger._tippy?.hide();
    this.trigger._tippy?.disable();
    this.popup.style.minWidth = `${this.trigger.offsetWidth}px`;
    this.refreshItems();
    if (this.loadItems) this.reload();
    this.tippy.show();
  }

  // tippy has positioned the popup, so focusing and scrolling to an item won't scroll the page
  onMount() {
    this.keyboardOwner.focus();
    const selected = this.kind === 'select' ? this.findItem(this.value) : null;
    if (selected) this.highlight(selected);
    this.updateState();
  }

  hide() {
    this.tippy?.hide();
  }

  onHidden(instance: Instance) {
    this.container.append(this.popup);
    instance.destroy();
    this.tippy = null;
    shownDropdowns.delete(this);
    if (!shownDropdowns.size) removalObserver.disconnect();
    this.trigger.setAttribute('aria-expanded', 'false');
    this.trigger._tippy?.enable();
    this.highlight(null);
    this.typeAhead = '';
    this.reloadDebounced.cancel();
    this.loadAbort?.abort();
    if (this.filterInput?.value) {
      this.filterInput.value = '';
      if (!this.loadItems) this.applyFilter();
    }
    this.container.dispatchEvent(new CustomEvent('ce-dropdown-hide'));
  }

  onPopupKeyDown(e: KeyboardEvent) {
    if (e.isComposing) return;
    const onOwner = e.target === this.keyboardOwner;
    if (e.key === 'Escape') {
      this.hide();
    } else if (e.key === 'Tab') {
      this.onTab(e);
      return;
    } else if (onOwner && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      this.moveHighlight(e.key === 'ArrowDown' ? 1 : -1);
    } else if (onOwner && !this.filterInput && (e.key === 'Home' || e.key === 'End')) {
      const items = this.navigableItems();
      this.highlight((e.key === 'Home' ? items[0] : items.at(-1)) ?? null);
    } else if (onOwner && (e.key === 'PageDown' || e.key === 'PageUp')) {
      const items = this.navigableItems();
      const pageSize = Math.max(1, Math.floor(this.popup.clientHeight / (items[0]?.offsetHeight || 1)));
      const index = (this.highlighted ? items.indexOf(this.highlighted) : -1) + (e.key === 'PageDown' ? pageSize : -pageSize);
      this.highlight(items[Math.min(Math.max(index, 0), items.length - 1)] ?? null);
    } else if (onOwner && this.highlighted && this.popup.contains(this.highlighted) && (e.key === 'Enter' || (e.key === ' ' && !this.filterInput))) {
      this.highlighted.dispatchEvent(new MouseEvent('click', {
        bubbles: true, cancelable: true, view: window,
        altKey: e.altKey, ctrlKey: e.ctrlKey, metaKey: e.metaKey, shiftKey: e.shiftKey,
      }));
    } else if (onOwner && !this.filterInput && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      this.onTypeAhead(e.key);
    } else {
      return;
    }
    e.preventDefault();
  }

  // Tab moves through the controls in the popup, leaving them closes the popup like a native select
  onTab(e: KeyboardEvent) {
    const radioStops = new Map<string, HTMLInputElement>(); // a radio group is one tab stop: the focused, checked or first radio
    for (const radio of this.popup.querySelectorAll<HTMLInputElement>('input[type="radio"][name]')) {
      const stop = radioStops.get(radio.name);
      if (!stop || radio === e.target || (radio.checked && stop !== e.target)) radioStops.set(radio.name, radio);
    }
    const tabbables = Array.from(this.popup.querySelectorAll<HTMLElement>('input, button, select, textarea, a[href], [tabindex]'))
      .filter((el) => el.tabIndex >= 0 && !el.matches(':disabled, .dropdown-item') && isElemVisible(el))
      .filter((el) => !(el instanceof HTMLInputElement && el.type === 'radio' && el.name) || radioStops.get(el.name) === el);
    const index = tabbables.indexOf(e.target as HTMLElement);
    if (e.shiftKey ? index > 0 : index < tabbables.length - 1) return;
    this.trigger.focus();
    this.hide();
    if (e.shiftKey) e.preventDefault();
  }

  // like a native select: repeating one letter cycles through the items starting with it
  onTypeAhead(key: string) {
    const char = key.toLowerCase();
    this.typeAhead += char;
    this.resetTypeAhead();
    const items = this.navigableItems();
    const isCycling = this.typeAhead === char.repeat(this.typeAhead.length);
    const start = isCycling && this.highlighted ? items.indexOf(this.highlighted) + 1 : 0;
    const prefix = isCycling ? char : this.typeAhead;
    const match = [...items.slice(start), ...items.slice(0, start)].find((item) => item.textContent.trim().toLowerCase().startsWith(prefix));
    if (match) this.highlight(match);
  }

  onPopupClick(e: MouseEvent) {
    const item = (e.target as Element).closest<HTMLElement>('.dropdown-item');
    if (!item || !this.popup.contains(item)) return;
    if (!isItemEnabled(item)) {
      e.preventDefault();
      e.stopPropagation();
      return;
    }
    if (this.kind !== 'menu' && item.hasAttribute('data-value')) {
      this.pick(item.getAttribute('data-value')!);
      if (this.kind === 'multiselect') {
        if (this.filterInput?.value) {
          this.filterInput.value = '';
          this.onFilterInput();
        }
        return;
      }
    } else if (this.container.hasAttribute('data-keep-open')) {
      return;
    }
    this.hide();
  }

  onFilterInput() {
    if (!this.loadItems) {
      this.applyFilter();
      return;
    }
    this.loadAbort?.abort();
    this.highlight(null); // Enter must not pick a result of the previous query
    this.reloadDebounced();
  }

  applyFilter() {
    const query = this.filterInput!.value.trim().toLowerCase();
    for (const item of this.items()) {
      item.hidden = Boolean(query) && !item.textContent.toLowerCase().includes(query);
    }
    if (this.tippy) this.updateState();
  }

  highlight(item: HTMLElement | null, scroll = true) {
    this.highlighted?.removeAttribute('data-highlighted');
    this.highlighted = item;
    if (!item) {
      this.keyboardOwner.removeAttribute('aria-activedescendant');
      return;
    }
    item.id ||= generateElemId('_dropdown_item_');
    item.setAttribute('data-highlighted', '');
    this.keyboardOwner.setAttribute('aria-activedescendant', item.id);
    if (scroll) item.scrollIntoView({block: 'nearest'});
  }

  moveHighlight(delta: number, items = this.navigableItems()) {
    if (!items.length) return;
    const index = this.highlighted ? items.indexOf(this.highlighted) : -1;
    const next = index === -1 ? (delta > 0 ? 0 : items.length - 1) : (index + delta + items.length) % items.length;
    this.highlight(items[next]);
  }

  // user selection, fires "change" when the value changes
  pick(value: string) {
    const values = this.values();
    let newValue = value;
    if (this.kind === 'multiselect' && value) {
      newValue = (values.includes(value) ? values.filter((selected) => selected !== value) : [...values, value]).join(this.delimiter);
    }
    if (newValue === this.value) return;
    this.setValue(newValue);
    this.valueInput!.dispatchEvent(new Event('change', {bubbles: true}));
  }

  setValue(value: string | string[]) {
    this.valueInput!.value = Array.isArray(value) ? value.join(this.delimiter) : value;
    if (this.tippy) this.refreshItems();
    this.renderLabel();
  }

  setActiveItem(item: HTMLElement) {
    for (const option of this.items()) option.classList.toggle('active', option === item);
    this.renderLabel(item);
  }

  renderLabel(item = this.findItem(this.value)) {
    const textEl = this.trigger.querySelector('.dropdown-text');
    if (!textEl) return;
    const placeholder = textEl.getAttribute('data-placeholder') ?? '';
    if (this.kind === 'multiselect') {
      const labels = this.values().map((value) => {
        const item = this.findItem(value);
        return item ? item.getAttribute('data-text') ?? item.textContent.trim() : value;
      });
      textEl.textContent = labels.join(', ') || placeholder;
      return;
    }
    const itemText = item?.getAttribute('data-text');
    if (item?.getAttribute('data-value') === '' && textEl.hasAttribute('data-placeholder')) {
      textEl.textContent = placeholder;
    } else if (itemText) {
      textEl.textContent = itemText;
    } else if (item) {
      textEl.innerHTML = item.innerHTML;
    } else if (!this.value) {
      textEl.textContent = placeholder;
    }
  }

  refreshItems() {
    const isMenu = this.kind === 'menu';
    const values = isMenu ? [] : this.values();
    for (const item of this.items()) {
      item.tabIndex = -1;
      if (!item.hasAttribute('role')) item.setAttribute('role', isMenu ? 'menuitem' : 'option');
      const value = item.getAttribute('data-value');
      if (!isMenu && value !== null) item.setAttribute('aria-selected', String(values.includes(value)));
    }
  }

  // replace the items, the ones marked with "data-static" stay
  setItems(items: Element[]) {
    this.highlight(null);
    for (const el of this.popup.querySelectorAll(':scope > :is(.dropdown-item, .dropdown-divider, .dropdown-header):not([data-static])')) {
      el.remove();
    }
    this.popup.append(...items);
    if (this.kind === 'select') this.renderLabel();
    if (this.tippy) {
      this.refreshItems();
      this.updateState();
    }
  }

  async reload() {
    this.reloadDebounced.cancel();
    this.loadAbort?.abort();
    const abort = new AbortController();
    this.loadAbort = abort;
    this.popup.setAttribute('aria-busy', 'true');
    try {
      const items = await this.loadItems!(this.filterInput?.value.trim() ?? '', abort.signal);
      if (abort.signal.aborted) return;
      this.popup.removeAttribute('aria-busy');
      this.setItems(items);
    } catch (err) {
      if (abort.signal.aborted) return;
      this.popup.removeAttribute('aria-busy');
      console.error(err);
      this.updateState(window.config.i18n.error_occurred);
    }
  }

  updateState(message = '') {
    hideScopedEmptyDividers(this.popup);
    this.popup.querySelector(':scope > .dropdown-message')?.remove();
    const allItems = this.items();
    const items = this.navigableItems(allItems);
    if (this.highlighted && !items.includes(this.highlighted)) this.highlight(null);
    if (!this.highlighted && (this.filterInput?.value.trim() || this.highlightDelta)) this.moveHighlight(this.highlightDelta || 1, items);
    if (!message && !this.popup.hasAttribute('aria-busy') && !items.length && allItems.every((item) => isItemEnabled(item) || !isElemVisible(item))) message = window.config.i18n.no_results_found;
    if (message) this.popup.append(createElementFromAttrs('div', {class: 'dropdown-message'}, message));
    this.tippy?.popper.style.removeProperty('--tippy-max-height'); // measure the new content unconstrained to pick the roomier side
    this.tippy?.popperInstance?.update();
  }
}

export function initDropdown(container: HTMLElement, opts?: DropdownOptions): Dropdown {
  const dropdown = instances.get(container);
  if (dropdown && opts) throw new Error('initDropdown: options must be passed on the first call');
  return dropdown ?? new Dropdown(container, opts ?? {});
}

// resolves the dropdown of an element inside its container or popup, also while the popup is portalled
export function closestDropdown(el: Element): Dropdown | null {
  const elDropdown = el.closest('.dropdown-popup, .dropdown-container');
  return elDropdown ? instances.get(elDropdown) ?? null : null;
}

// hides dividers that separate nothing, a "data-scope" divider hides together with its filtered out group
export function hideScopedEmptyDividers(container: Element) {
  const visibleItems: HTMLElement[] = [];
  const curScopeVisibleItems: HTMLElement[] = [];
  let curScope: string = '', lastVisibleScope: string = '';
  const isDivider = (item: Element) => item.classList.contains('dropdown-divider');
  const isScopedDivider = (item: Element) => isDivider(item) && item.hasAttribute('data-scope');
  const isHidden = (item: HTMLElement) => item.hidden || item.classList.contains('tw-hidden');
  const handleScopeSwitch = (itemScope: string) => {
    const first = curScopeVisibleItems[0];
    if (first && isScopedDivider(first) && (curScopeVisibleItems.length === 1 || lastVisibleScope === curScope)) {
      first.hidden = true;
      curScopeVisibleItems.shift();
    }
    if (curScopeVisibleItems.length) {
      visibleItems.push(...curScopeVisibleItems);
      lastVisibleScope = curScope;
    }
    curScope = itemScope;
    curScopeVisibleItems.length = 0;
  };

  for (const item of container.querySelectorAll<HTMLElement>(':scope > :is(.dropdown-item, .dropdown-divider)')) {
    if (isDivider(item)) item.hidden = false;
    const itemScope = item.getAttribute('data-scope') || '';
    if (itemScope !== curScope) handleScopeSwitch(itemScope);
    if (!isHidden(item)) curScopeVisibleItems.push(item);
  }
  handleScopeSwitch('');

  while (visibleItems.length && isDivider(visibleItems[0])) {
    visibleItems.shift()!.hidden = true;
  }
  while (visibleItems.length && isDivider(visibleItems.at(-1)!)) {
    visibleItems.pop()!.hidden = true;
  }
  for (let i = 0; i < visibleItems.length - 1; i++) {
    if (isDivider(visibleItems[i]) && isDivider(visibleItems[i + 1])) visibleItems[i].hidden = true;
  }
}
