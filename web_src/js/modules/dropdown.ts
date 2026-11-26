import {createTippy} from './tippy.ts';
import {
  addDelegatedEventListener, createElementFromAttrs, createElementFromHTML, generateElemId, isElemVisible, onInputDebounce,
} from '../utils/dom.ts';
import {svg} from '../svg.ts';
import {trString} from './i18n.ts';
import type {Instance, Placement} from 'tippy.js';

type DropdownKind = 'menu' | 'select' | 'multiselect';

type DropdownOptions = {
  loadItems?: (query: string, signal: AbortSignal) => Promise<Element[]>,
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
  combobox: HTMLElement;
  kind: DropdownKind;
  delimiter: string;
  loadItems: DropdownOptions['loadItems'];
  tippy: Instance | null = null;
  highlighted: HTMLElement | null = null;
  highlightDelta = 0;
  userNavigated = false;
  loadAbort: AbortController | null = null;
  reloadDebounced = onInputDebounce(() => this.reload());

  constructor(container: HTMLElement, opts: DropdownOptions) {
    this.container = container;
    this.trigger = container.querySelector(':scope > .dropdown-trigger')!;
    this.popup = container.querySelector(':scope > .dropdown-popup')!;
    this.valueInput = container.querySelector(':scope > .dropdown-value');
    this.filterInput = container.querySelector('.dropdown-filter');
    this.combobox = this.trigger.contains(this.filterInput) ? this.filterInput! : this.trigger;
    this.kind = container.getAttribute('data-kind') === 'multiselect' ? 'multiselect' : (this.valueInput ? 'select' : 'menu');
    this.delimiter = container.getAttribute('data-delimiter') || ',';
    this.loadItems = opts.loadItems;
    instances.set(container, this);
    instances.set(this.popup, this);

    const isMenu = this.kind === 'menu';
    this.popup.id ||= generateElemId('_dropdown_popup_');
    this.popup.tabIndex = -1;
    if (!this.popup.hasAttribute('role')) this.popup.setAttribute('role', isMenu ? 'menu' : 'listbox');
    if (this.kind === 'multiselect') this.popup.setAttribute('aria-multiselectable', 'true');
    const combobox = this.combobox;
    combobox.setAttribute('aria-haspopup', isMenu ? 'menu' : 'listbox');
    combobox.setAttribute('aria-controls', this.popup.id);
    combobox.setAttribute('aria-expanded', 'false');
    this.trigger.id ||= generateElemId('_dropdown_trigger_');
    const fieldLabel = container.parentElement!.querySelector(':scope.field > label');
    if (fieldLabel) {
      fieldLabel.id ||= generateElemId('_dropdown_label_');
      const valueEl = this.trigger.querySelector('.dropdown-text') ?? this.trigger;
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
        if (this.filterInput !== combobox) this.filterInput.setAttribute('aria-expanded', 'true');
      }
    }
    if (this.valueInput) this.renderLabel();

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
        this.show(e.detail === 0 ? 1 : 0);
      }
    });
    this.trigger.addEventListener('keydown', (e) => {
      if (e.isComposing) return;
      if (this.tippy) {
        this.onPopupKeyDown(e);
      } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || (e.key === 'Enter' && e.target === this.filterInput)) {
        e.preventDefault();
        this.show(e.key === 'ArrowUp' ? -1 : 1);
      }
    });
    this.popup.addEventListener('keydown', (e) => this.onPopupKeyDown(e));
    addDelegatedEventListener(this.popup, 'click', '.dropdown-item', (item: HTMLElement, e: MouseEvent) => this.onItemClick(item, e), {capture: true}); // close before item handlers run, so they see the popup in its container
    this.popup.addEventListener('mousedown', (e) => {
      this.popup.removeAttribute('data-keyboard-nav');
      if (!(e.target as Element).closest('input, textarea, select')) e.preventDefault();
    });
    this.filterInput?.addEventListener('input', () => this.onFilterInput());
    this.filterInput?.addEventListener('keydown', (e) => {
      if (e.key !== 'Backspace' || e.isComposing || this.kind !== 'multiselect' || this.filterInput!.value) return;
      const lastValue = this.values().at(-1);
      if (lastValue) this.pick(lastValue);
    });
    for (const el of [this.container, this.popup]) {
      el.addEventListener('focusout', (e) => {
        const related = e.relatedTarget as Node | null;
        if (related && !this.container.contains(related) && !this.popup.contains(related)) this.leave();
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

  items(): HTMLElement[] {
    return Array.from(this.popup.querySelectorAll<HTMLElement>('.dropdown-item'));
  }

  navigableItems(): HTMLElement[] {
    if (this.popup.hasAttribute('aria-busy')) return []; // rows of a superseded query must not be picked
    return this.items().filter((item) => isItemEnabled(item) && isElemVisible(item));
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
    if (highlightDelta) this.popup.setAttribute('data-keyboard-nav', '');
    this.tippy = createTippy(this.container, {
      content: this.popup,
      theme: 'dropdown',
      role: 'presentation',
      aria: {content: null, expanded: false},
      trigger: 'manual',
      interactive: true,
      arrow: false,
      maxWidth: 'none',
      placement: (this.container.getAttribute('data-placement') || 'bottom-start') as Placement,
      appendTo: this.container.closest('.modal-backdrop') ?? document.body, // outside the backdrop, the page is inert
      zIndex: 1000,
      limitSizeToViewport: {vertical: true},
      onMount: () => this.onMount(),
      onClickOutside: () => this.leave(),
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
    this.popup.style[this.trigger.matches('.dropdown-field') ? 'width' : 'minWidth'] = `min(${this.trigger.offsetWidth}px, 100vw - 32px)`;
    this.refreshItems();
    this.tippy.show();
  }

  onMount() {
    this.keyboardOwner.focus();
    if (!this.loadItems && this.filterInput?.value) return this.applyFilter();
    const selected = this.kind === 'select' ? this.findItem(this.value) : null;
    if (selected) {
      this.highlight(selected);
    } else if (this.kind === 'multiselect') {
      this.moveHighlight(this.highlightDelta || 1);
    }
    if (this.loadItems) this.reload();
    this.updateState();
  }

  hide() {
    this.tippy?.hide();
  }

  leave() { // a search field keeps what the user typed or arrowed to like an autocomplete, popup pickers cancel
    if (this.kind === 'select' && this.combobox === this.filterInput && this.userNavigated && this.highlighted) this.pick(this.highlighted.getAttribute('data-value')!);
    this.hide();
  }

  onHidden(instance: Instance) {
    this.container.append(this.popup);
    instance.destroy();
    this.tippy = null;
    this.userNavigated = false;
    shownDropdowns.delete(this);
    this.container.removeAttribute('data-shown');
    if (!shownDropdowns.size) removalObserver.disconnect();
    this.combobox.setAttribute('aria-expanded', 'false');
    this.trigger._tippy?.enable();
    this.highlight(null);
    this.popup.removeAttribute('data-keyboard-nav');
    this.reloadDebounced.cancel();
    this.loadAbort?.abort();
    this.popup.removeAttribute('aria-busy');
    if (this.filterInput) {
      this.filterInput.value = this.filterInput.defaultValue;
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
      this.userNavigated = true;
      this.moveHighlight(e.key === 'ArrowDown' ? 1 : -1);
    } else if (onOwner && (e.key === 'Enter' || (e.key === ' ' && !this.filterInput))) {
      if (this.highlighted) {
        this.highlighted.dispatchEvent(new MouseEvent('click', {
          bubbles: true, cancelable: true, view: window,
          altKey: e.altKey, ctrlKey: e.ctrlKey, metaKey: e.metaKey, shiftKey: e.shiftKey,
        }));
      }
    } else {
      return;
    }
    e.preventDefault();
  }

  onTab(e: KeyboardEvent) {
    const target = e.target as HTMLElement;
    if (!this.popup.contains(target)) return;
    const radioName = target instanceof HTMLInputElement && target.type === 'radio' ? target.name : '';
    const tabbables = Array.from(this.popup.querySelectorAll<HTMLElement>('input, button, select, textarea, a[href], [tabindex]'))
      .filter((el) => el.tabIndex >= 0 && !el.matches(':disabled, .dropdown-item') && isElemVisible(el))
      .filter((el) => el === target || !radioName || (el as HTMLInputElement).name !== radioName); // a radio group is a single tab stop
    const index = tabbables.indexOf(target);
    if (e.shiftKey ? index > 0 : index < tabbables.length - 1) return;
    this.combobox.focus();
    this.leave();
    if (e.shiftKey && target !== this.popup) e.preventDefault();
  }

  onItemClick(item: HTMLElement, e: MouseEvent) {
    if (!isItemEnabled(item)) {
      e.preventDefault();
      e.stopPropagation();
      return;
    }
    if (this.kind !== 'menu') this.pick(item.getAttribute('data-value')!);
    if (this.kind === 'multiselect') {
      if (this.filterInput?.value) {
        this.filterInput.value = '';
        this.onFilterInput();
      }
    } else if (!this.container.hasAttribute('data-keep-open')) {
      this.hide();
    }
  }

  onFilterInput() {
    this.userNavigated = Boolean(this.filterInput!.value.trim());
    this.trigger.classList.toggle('filtering', Boolean(this.filterInput!.value));
    if (!this.tippy) return this.show(); // onMount filters, highlighting earlier would scroll the page to the unpositioned popup
    if (!this.loadItems) return this.applyFilter();
    this.loadAbort?.abort();
    this.highlight(null);
    this.popup.setAttribute('aria-busy', 'true');
    this.reloadDebounced();
  }

  applyFilter() {
    const query = this.filterInput!.value.trim().toLowerCase();
    let groupHasMatch = false;
    let belowIsItem = true;
    const elements = this.popup.querySelectorAll<HTMLElement>('.dropdown-item, .dropdown-header, .dropdown-divider');
    for (const el of Array.from(elements).reverse()) { // reversed, so each header and divider knows whether the items below it match
      const isItem = el.matches('.dropdown-item');
      if (isItem) {
        el.hidden = Boolean(query) && !el.textContent.toLowerCase().includes(query);
        groupHasMatch = (belowIsItem && groupHasMatch) || (!el.hidden && !el.classList.contains('tw-hidden'));
      } else {
        el.hidden = Boolean(query) && !groupHasMatch;
      }
      belowIsItem = isItem;
    }
    if (!this.tippy) return;
    this.highlight(query ? this.navigableItems()[0] ?? null : null);
    this.updateState();
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

  pick(value: string) {
    let newValue = value;
    if (this.kind === 'multiselect') {
      const values = this.values();
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
      this.trigger.prepend(...this.values().map((value) => this.createValueLabel(value)));
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
    const label = createElementFromAttrs('span', {class: 'ui label', 'data-value': value});
    if (item) {
      label.innerHTML = item.innerHTML;
    } else {
      label.textContent = value;
    }
    label.append(createElementFromAttrs('button', {
      type: 'button', class: 'delete icon btn',
      'aria-label': trString(window.config.i18n.remove_label_str, label.textContent.trim()),
    }, createElementFromHTML(svg('octicon-x', 16))));
    return label;
  }

  refreshItems() {
    const isMenu = this.kind === 'menu';
    const values = new Set(isMenu ? [] : this.values());
    for (const item of this.items()) {
      item.tabIndex = -1;
      if (!item.hasAttribute('role')) item.setAttribute('role', isMenu ? 'menuitem' : 'option');
      if (!isMenu) item.setAttribute('aria-selected', String(values.has(item.getAttribute('data-value')!)));
    }
  }

  setItems(items: Element[]) {
    this.highlight(null);
    for (const el of this.popup.querySelectorAll(':scope > :is(.dropdown-item, .dropdown-divider, .dropdown-header):not([data-static])')) {
      el.remove();
    }
    this.popup.append(...items);
    if (this.kind === 'select') this.renderLabel();
    if (this.tippy) {
      this.tippy.popper.style.removeProperty('--tippy-max-height'); // flip by the uncapped height of the new items
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
    this.popup.querySelector(':scope > .dropdown-message')?.remove();
    const isBusy = this.popup.hasAttribute('aria-busy');
    const visibleItems = this.items().filter((item) => isElemVisible(item));
    const items = visibleItems.filter(isItemEnabled);
    if (this.highlighted && !items.includes(this.highlighted)) this.highlight(null);
    if (!this.highlighted && !isBusy && (this.highlightDelta || this.loadItems)) {
      const active = items.find((item) => item.matches('.active, [aria-selected="true"]'));
      if (active) this.highlight(active);
      else this.moveHighlight(this.highlightDelta || 1, items);
    }
    if (!message && !isBusy && !visibleItems.length) message = window.config.i18n.no_results_found;
    if (message) this.popup.append(createElementFromAttrs('div', {class: 'dropdown-message'}, message));
    this.tippy?.popperInstance?.update();
  }
}

export function initDropdown(container: HTMLElement, opts?: DropdownOptions): Dropdown {
  const dropdown = instances.get(container);
  if (dropdown && opts) throw new Error('initDropdown: options must be passed on the first call');
  return dropdown ?? new Dropdown(container, opts ?? {});
}

export function closestDropdown(el: Element): Dropdown | null {
  const elDropdown = el.closest('.dropdown-popup, .dropdown-container');
  return elDropdown ? instances.get(elDropdown) ?? null : null;
}
