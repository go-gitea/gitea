import {createTippy} from './tippy.ts';
import {
  addDelegatedEventListener, createElementFromAttrs, createElementFromHTML, generateElemId, isElemVisible, onInputDebounce,
} from '../utils/dom.ts';
import {debounce} from '../utils/func.ts';
import {svg} from '../svg.ts';
import {trString} from './i18n.ts';
import type {Instance, Placement} from 'tippy.js';

type DropdownKind = 'menu' | 'select' | 'multiselect';

type DropdownOptions = {
  loadItems?: (query: string, signal: AbortSignal) => Promise<Element[]>, // remote items, loaded on every open and while filtering
};

const instances = new WeakMap<Element, Dropdown>();
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
  filterInField: boolean; // a search field: the filter input sits in the trigger and is the combobox
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
    this.filterInput = container.querySelector('.dropdown-filter');
    this.filterInField = Boolean(this.filterInput && this.trigger.contains(this.filterInput));
    this.kind = (container.getAttribute('data-kind') || 'menu') as DropdownKind;
    this.delimiter = container.getAttribute('data-delimiter') || ',';
    this.loadItems = opts.loadItems;
    instances.set(container, this);
    instances.set(this.popup, this);

    const isMenu = this.kind === 'menu';
    this.popup.id ||= generateElemId('_dropdown_popup_');
    this.popup.tabIndex = -1;
    if (this.trigger.classList.contains('dropdown-field')) this.popup.classList.add('dropdown-popup-field');
    if (!this.popup.hasAttribute('role')) this.popup.setAttribute('role', isMenu ? 'menu' : 'listbox');
    if (this.kind === 'multiselect') this.popup.setAttribute('aria-multiselectable', 'true');
    const combobox = this.combobox;
    combobox.setAttribute('aria-haspopup', isMenu ? 'menu' : 'listbox');
    combobox.setAttribute('aria-controls', this.popup.id);
    combobox.setAttribute('aria-expanded', 'false');
    this.trigger.id ||= generateElemId('_dropdown_trigger_');
    const fieldLabel = container.parentElement!.querySelector(':scope.field > label');
    if (fieldLabel) { // like a native select: the field label, then the value
      fieldLabel.id ||= generateElemId('_dropdown_label_');
      const valueEl = this.filterInField ? this.trigger.querySelector('.dropdown-text')! : this.trigger;
      valueEl.id ||= generateElemId('_dropdown_text_');
      combobox.setAttribute('aria-labelledby', `${fieldLabel.id} ${valueEl.id}`);
    }
    this.popup.setAttribute('aria-labelledby', combobox.getAttribute('aria-labelledby') ?? this.trigger.id);
    if (this.filterInput) {
      this.filterInput.autocomplete = 'off';
      this.filterInput.setAttribute('aria-controls', this.popup.id);
      if (!isMenu) {
        this.filterInput.setAttribute('role', 'combobox');
        this.filterInput.setAttribute('aria-autocomplete', 'list');
        if (!this.filterInField) this.filterInput.setAttribute('aria-expanded', 'true');
      }
    }
    if (this.valueInput) this.renderLabel(); // the browser may have restored a previous value

    this.trigger.addEventListener('click', (e) => {
      const target = e.target as Element;
      const label = target.closest<HTMLElement>('.ui.label[data-value]');
      if (label) {
        if (target.closest('.delete.icon')) {
          this.pick(label.getAttribute('data-value')!);
          this.combobox.focus();
        }
        return;
      }
      if (this.tippy) {
        if (target !== this.filterInput) this.hide();
      } else {
        this.show(e.detail === 0 ? 1 : 0); // zero clicks means Enter or Space
      }
    });
    this.trigger.addEventListener('keydown', (e) => {
      const label = (e.target as Element).closest('.ui.label[data-value]');
      if (label && ['Enter', ' ', 'Backspace', 'Delete'].includes(e.key)) {
        e.preventDefault();
        this.pick(label.getAttribute('data-value')!);
        this.combobox.focus();
      } else if (e.key === 'Backspace' && this.kind === 'multiselect' && !this.filterInput?.value) {
        const values = this.values();
        if (values.length) this.pick(values.at(-1)!);
      } else if (this.tippy && this.filterInField) {
        this.onPopupKeyDown(e);
      } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        this.show(e.key === 'ArrowDown' ? 1 : -1);
      }
    });
    this.popup.addEventListener('keydown', (e) => this.onPopupKeyDown(e));
    addDelegatedEventListener(this.popup, 'click', '.dropdown-item', (item: HTMLElement, e: MouseEvent) => this.onItemClick(item, e), {capture: true}); // close before item handlers run, so they see the popup in its container
    this.popup.addEventListener('mousedown', (e) => { // keep the focus on the filter input or the popup
      this.popup.removeAttribute('data-keyboard-nav');
      if (!(e.target as Element).closest('input, textarea, select')) e.preventDefault();
    });
    this.filterInput?.addEventListener('input', () => this.onFilterInput());
    for (const el of [this.container, this.popup]) {
      el.addEventListener('focusout', (e) => {
        const related = e.relatedTarget as Node | null;
        if (related && !this.container.contains(related) && !this.popup.contains(related)) this.hide();
      });
    }
  }

  get keyboardOwner(): HTMLElement {
    return this.filterInput ?? this.popup;
  }

  get combobox(): HTMLElement {
    return this.filterInField ? this.filterInput! : this.trigger;
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

  items(): HTMLElement[] {
    return Array.from(this.popup.querySelectorAll<HTMLElement>('.dropdown-item'));
  }

  navigableItems(): HTMLElement[] {
    return this.items().filter((item) => isItemEnabled(item) && isElemVisible(item));
  }

  findItem(value: string): HTMLElement | null {
    return this.popup.querySelector<HTMLElement>(`.dropdown-item[data-value="${CSS.escape(value)}"]`);
  }

  show(highlightDelta = 0) {
    if (this.tippy || this.trigger.disabled) return;
    if (this.filterInput?.value && !this.loadItems) this.applyFilter(); // the browser may have restored a previous query
    for (const control of this.popup.querySelectorAll('input:not(.dropdown-filter), button, select, textarea')) {
      const form = control.closest('form');
      if (!form || control.hasAttribute('form')) continue;
      form.id ||= generateElemId('_dropdown_form_');
      control.setAttribute('form', form.id);
    }
    this.highlightDelta = highlightDelta;
    if (highlightDelta) this.popup.setAttribute('data-keyboard-nav', '');
    this.popup.classList.toggle('dropdown-popup-error', Boolean(this.container.closest('.field.error')));
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
      zIndex: 1000, // below --z-index-modal and --z-index-toast
      limitSizeToViewport: {vertical: true},
      onMount: () => this.onMount(),
      onClickOutside: () => this.hide(),
      onHide: () => {
        if (this.popup.contains(document.activeElement)) this.combobox.focus();
      },
      onHidden: (instance) => this.onHidden(instance),
    });
    if (!shownDropdowns.size) removalObserver.observe(document, {subtree: true, childList: true});
    shownDropdowns.add(this);
    this.container.setAttribute('data-shown', '');
    this.combobox.setAttribute('aria-expanded', 'true');
    this.trigger._tippy?.hide();
    this.trigger._tippy?.disable();
    const {clientWidth} = this.trigger; // the min widths of the former menus, relative to their dropdown's padding box
    const minWidth = this.trigger.classList.contains('dropdown-field') ? clientWidth + 2 : this.popup.classList.contains('dropdown-popup-button') ? clientWidth :
      this.popup.matches('.dropdown-popup-menu, .dropdown-popup-small') ? clientWidth - 1 : 0;
    if (minWidth) this.popup.style.minWidth = `min(${minWidth}px, 100vw - 32px)`; // the max width of [data-tippy-root]
    this.refreshItems();
    this.tippy.show();
  }

  // tippy has positioned the popup, so focusing and scrolling to an item won't scroll the page
  onMount() {
    this.keyboardOwner.focus();
    const selected = this.kind === 'select' ? this.findItem(this.value) : null;
    if (selected) {
      this.highlight(selected);
    } else if (this.kind === 'multiselect' && this.popup.classList.contains('dropdown-popup-field')) {
      this.moveHighlight(1);
    }
    if (this.loadItems) this.reload();
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
    this.container.removeAttribute('data-shown');
    if (!shownDropdowns.size) removalObserver.disconnect();
    this.combobox.setAttribute('aria-expanded', 'false');
    this.trigger._tippy?.enable();
    this.highlight(null);
    this.popup.removeAttribute('data-keyboard-nav');
    this.trigger.removeAttribute('data-popup-side');
    this.typeAhead = '';
    this.reloadDebounced.cancel();
    this.loadAbort?.abort();
    if (this.filterInput?.value) {
      this.filterInput.value = '';
      this.trigger.classList.remove('filtering');
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
      this.popup.setAttribute('data-keyboard-nav', '');
      this.moveHighlight(e.key === 'ArrowDown' ? 1 : -1);
    } else if (onOwner && !this.filterInput && (e.key === 'Home' || e.key === 'End')) {
      const items = this.navigableItems();
      this.popup.setAttribute('data-keyboard-nav', '');
      this.highlight((e.key === 'Home' ? items[0] : items.at(-1)) ?? null);
    } else if (onOwner && (e.key === 'Enter' || (e.key === ' ' && !this.filterInput))) { // without a highlight, Enter must not submit the form
      if (this.highlighted && this.popup.contains(this.highlighted)) {
        this.highlighted.dispatchEvent(new MouseEvent('click', {
          bubbles: true, cancelable: true, view: window,
          altKey: e.altKey, ctrlKey: e.ctrlKey, metaKey: e.metaKey, shiftKey: e.shiftKey,
        }));
      }
    } else if (onOwner && !this.filterInput && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      this.onTypeAhead(e.key);
    } else {
      return;
    }
    e.preventDefault();
  }

  // Tab moves through the controls in the popup, leaving them closes the popup like a native select
  onTab(e: KeyboardEvent) {
    if (!this.popup.contains(e.target as Node)) return; // tabbing out of a search field closes the popup on focusout
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
    this.combobox.focus();
    this.hide();
    if (e.shiftKey && e.target !== this.popup) e.preventDefault();
  }

  onTypeAhead(key: string) {
    this.typeAhead += key.toLowerCase();
    this.resetTypeAhead();
    const match = this.navigableItems().find((item) => item.textContent.trim().toLowerCase().startsWith(this.typeAhead));
    if (!match) return;
    this.popup.setAttribute('data-keyboard-nav', '');
    this.highlight(match);
  }

  onItemClick(item: HTMLElement, e: MouseEvent) {
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
    if (this.filterInField) {
      this.trigger.classList.toggle('filtering', Boolean(this.filterInput!.value));
      if (!this.tippy) this.show();
    }
    if (!this.loadItems) return this.applyFilter();
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

  highlight(item: HTMLElement | null) {
    this.highlighted?.removeAttribute('data-highlighted');
    this.highlighted = item;
    if (!item) {
      this.keyboardOwner.removeAttribute('aria-activedescendant');
      return;
    }
    item.id ||= generateElemId('_dropdown_item_');
    item.setAttribute('data-highlighted', '');
    this.keyboardOwner.setAttribute('aria-activedescendant', item.id);
    item.scrollIntoView({block: 'nearest'});
  }

  moveHighlight(delta: number, items = this.navigableItems()) {
    if (!items.length) return;
    const index = this.highlighted ? items.indexOf(this.highlighted) : -1;
    const next = index === -1 ? (delta > 0 ? 0 : items.length - 1) : Math.min(Math.max(index + delta, 0), items.length - 1);
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
    if (this.tippy) {
      this.refreshItems();
      this.updateState();
    }
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
      for (const label of this.trigger.querySelectorAll(':scope > .ui.label[data-value]')) label.remove();
      (this.filterInField ? this.filterInput! : textEl).before(...this.values().map((value) => this.createValueLabel(value)));
      textEl.textContent = placeholder;
      return;
    }
    const itemText = item?.getAttribute('data-text');
    if ((item?.getAttribute('data-value') === '' && textEl.hasAttribute('data-placeholder')) || (!item && !this.value)) {
      textEl.textContent = placeholder;
    } else if (itemText) {
      textEl.textContent = itemText;
    } else if (item) {
      textEl.innerHTML = item.innerHTML;
    }
  }

  createValueLabel(value: string): HTMLElement {
    const item = this.findItem(value);
    const label = createElementFromAttrs('a', {class: 'ui label', 'data-value': value});
    if (item?.hasAttribute('data-text')) {
      label.textContent = item.getAttribute('data-text');
    } else if (item) {
      label.innerHTML = item.innerHTML;
    } else {
      label.textContent = value;
    }
    const deleteIcon = createElementFromHTML(svg('octicon-x', 16, 'delete icon'));
    deleteIcon.removeAttribute('aria-hidden');
    deleteIcon.setAttribute('aria-label', trString(window.config.i18n.remove_label_str, label.textContent.trim()));
    deleteIcon.setAttribute('role', 'button');
    if (!(this.trigger instanceof HTMLButtonElement)) deleteIcon.setAttribute('tabindex', '0'); // a button trigger can't contain focusables
    label.append(deleteIcon);
    return label;
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
    hideScopedEmptyDividers(this.popup.querySelector(':scope > .dropdown-list') ?? this.popup);
    this.popup.querySelector(':scope > .dropdown-message')?.remove();
    const visibleItems = this.items().filter((item) => isElemVisible(item));
    const items = visibleItems.filter(isItemEnabled);
    if (this.highlighted && !items.includes(this.highlighted)) this.highlight(null);
    if (!this.highlighted && (this.highlightDelta || this.loadItems)) { // remote results keep Enter on their active or first item
      const active = items.find((item) => item.matches('.active, [aria-selected="true"]'));
      if (active) this.highlight(active);
      else if (!this.popup.hasAttribute('aria-busy')) this.moveHighlight(this.highlightDelta || 1, items);
    }
    const side = this.tippy?.popper.firstElementChild?.getAttribute('data-placement')?.split('-')[0];
    if (side) this.trigger.setAttribute('data-popup-side', side);
    if (!message && !this.popup.hasAttribute('aria-busy') && !visibleItems.length) message = window.config.i18n.no_results_found;
    if (message) this.popup.append(createElementFromAttrs('div', {class: 'dropdown-message'}, message));
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

// hides dividers that separate nothing and headers of filtered out items, a "data-scope" divider hides together with its group
export function hideScopedEmptyDividers(container: Element) {
  const visibleItems: HTMLElement[] = [];
  const curScopeVisibleItems: HTMLElement[] = [];
  let curScope: string = '', lastVisibleScope: string = '';
  const isDivider = (item: Element) => item.classList.contains('dropdown-divider');
  const isScopedDivider = (item: Element) => isDivider(item) && item.hasAttribute('data-scope');
  const isHidden = (item: HTMLElement) => item.hidden || item.classList.contains('tw-hidden');
  const children = container.querySelectorAll<HTMLElement>(':scope > :not(.dropdown-message)');
  let headerHasItems = false, headerHasVisibleItems = false;
  for (const item of Array.from(children).reverse()) {
    if (isDivider(item)) {
      item.hidden = false;
    } else if (item.classList.contains('dropdown-header')) {
      item.hidden = headerHasItems && !headerHasVisibleItems;
      headerHasItems = headerHasVisibleItems = false;
    } else if (item.classList.contains('dropdown-item')) {
      headerHasItems = true;
      if (!isHidden(item)) headerHasVisibleItems = true;
    }
  }
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

  for (const item of children) {
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
