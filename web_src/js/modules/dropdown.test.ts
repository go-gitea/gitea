import {userEvent} from 'vitest/browser';
import {createElementFromAttrs, createElementFromHTML} from '../utils/dom.ts';
import {closestDropdown, initDropdown} from './dropdown.ts';

function createFormDropdown(html: string, opts?: Parameters<typeof initDropdown>[1]) {
  const form = createElementFromHTML<HTMLFormElement>(`<form>${html}</form>`);
  document.body.append(form);
  return {form, dropdown: initDropdown(form.querySelector<HTMLElement>('.dropdown-container')!, opts)};
}

test('select', async () => {
  const field = createElementFromHTML(`<div class="field"><label>Fruit</label><div class="dropdown-container">
    <button type="button" class="dropdown-trigger"><span class="dropdown-text" data-placeholder="Pick">Apple</span></button>
    <input type="hidden" class="dropdown-value" value="apple">
    <div class="dropdown-popup">
      <input class="dropdown-filter">
      <div class="dropdown-list">
        <div class="dropdown-item" data-value="">Clear</div>
        <div class="dropdown-divider"></div>
        <div class="dropdown-header">Fruits</div>
        <div class="dropdown-item" data-value="apple">Apple</div>
        <div class="dropdown-item" data-value="banana" data-text="Banana!"><b>Banana</b></div>
        <div class="dropdown-item" data-value="blueberry">Blueberry</div>
        <div class="dropdown-divider"></div>
        <div class="dropdown-item" data-value="carrot">Carrot</div>
      </div>
    </div>
  </div></div>`);
  document.body.append(field);
  const dropdown = initDropdown(field.querySelector<HTMLElement>('.dropdown-container')!);
  const changes: string[] = [];
  dropdown.valueInput!.addEventListener('change', () => { changes.push(dropdown.value) });

  await expect.element(dropdown.trigger).toHaveAccessibleName('Fruit Apple');
  await userEvent.click(dropdown.trigger);
  await expect.element(dropdown.popup).toHaveAccessibleName('Fruit Apple');
  expect(document.activeElement).toBe(dropdown.filterInput);
  expect(dropdown.highlighted!.textContent).toBe('Apple');
  await userEvent.keyboard('b{Enter}');
  expect(changes).toEqual(['banana']);
  expect(dropdown.trigger.textContent).toBe('Banana!');
  expect(dropdown.container.hasAttribute('data-shown')).toBe(false);
  expect(document.activeElement).toBe(dropdown.trigger);
  expect(dropdown.container.lastElementChild).toBe(dropdown.popup);
  expect(dropdown.items().some((item) => item.hidden)).toBe(false);

  await userEvent.keyboard('{ArrowDown}{Enter}');
  dropdown.setValue('apple');
  expect(changes).toEqual(['banana']);
  expect(dropdown.trigger.textContent).toBe('Apple');
  dropdown.setValue('');
  expect(dropdown.trigger.textContent).toBe('Pick');
  dropdown.setValue('blueberry');
  await userEvent.click(dropdown.trigger);
  await userEvent.keyboard('b');
  expect(dropdown.highlighted).toBe(dropdown.findItem('blueberry'));
  expect(dropdown.popup.querySelector<HTMLElement>('.dropdown-header')!.hidden).toBe(false);
  expect(Array.from(dropdown.popup.querySelectorAll<HTMLElement>('.dropdown-divider'), (el) => el.hidden)).toEqual([true, true]);
  await userEvent.keyboard('{Tab}');
  expect(changes).toEqual(['banana']);
  expect(dropdown.value).toBe('blueberry');
  expect(dropdown.container.hasAttribute('data-shown')).toBe(false);
  field.remove();
});

test('menu', async () => {
  const {form, dropdown} = createFormDropdown(`<div class="dropdown-container">
    <button type="button" class="dropdown-trigger">Menu</button>
    <div class="dropdown-popup">
      <input type="hidden" name="mode" value="all">
      <input type="radio" name="kind" value="a"><input type="radio" name="kind" value="b" checked>
      <button type="button" class="dropdown-item">Item</button>
      <div class="dropdown-item" aria-disabled="true">Disabled</div>
      <button type="button" class="dropdown-item">Other</button>
    </div>
  </div>`);
  const [item, disabledItem, otherItem] = dropdown.items();
  let itemContainer: Element | null = null;
  item.addEventListener('click', () => { itemContainer = item.closest('.dropdown-container') });

  await userEvent.click(dropdown.trigger);
  expect(form.contains(dropdown.popup)).toBe(false);
  expect(new FormData(form).get('mode')).toBe('all');
  expect(closestDropdown(item)).toBe(dropdown);
  disabledItem.click();
  expect(dropdown.container.hasAttribute('data-shown')).toBe(true);
  await userEvent.click(item);
  expect(itemContainer).toBe(dropdown.container);
  expect(dropdown.container.hasAttribute('data-shown')).toBe(false);

  await userEvent.keyboard('{ArrowUp}');
  expect(dropdown.highlighted).toBe(otherItem);
  await userEvent.keyboard('{ArrowUp}');
  expect(dropdown.highlighted).toBe(item);
  await userEvent.keyboard('{ArrowDown}');
  expect(dropdown.highlighted).toBe(otherItem);
  await userEvent.keyboard('{Escape}');
  expect(dropdown.container.hasAttribute('data-shown')).toBe(false);
  expect(document.activeElement).toBe(dropdown.trigger);
  await userEvent.keyboard('{Enter}');
  expect(dropdown.container.hasAttribute('data-shown')).toBe(true);
  dropdown.popup.querySelector<HTMLInputElement>(':checked')!.focus();
  await userEvent.keyboard('{Shift>}{Tab}{/Shift}');
  expect(dropdown.container.hasAttribute('data-shown')).toBe(false);
  dropdown.trigger.focus();
  await userEvent.keyboard(' ');
  expect(dropdown.container.hasAttribute('data-shown')).toBe(true);
  form.remove();
  await vi.waitFor(() => expect(dropdown.container.hasAttribute('data-shown')).toBe(false));
  expect(dropdown.popup.isConnected).toBe(false);
});

test('remote multiselect', async () => {
  const pending: Array<{query: string, resolve: (items: Element[]) => void}> = [];
  const {form, dropdown} = createFormDropdown(`<div class="dropdown-container" data-kind="multiselect">
    <input type="hidden" class="dropdown-value" value="">
    <div class="dropdown-trigger dropdown-field dropdown-search-field">
      <input class="dropdown-filter" type="text"><span class="dropdown-text"></span>
    </div>
    <div class="dropdown-popup">
      <div class="dropdown-item" data-value="old">Old</div>
    </div>
  </div>`, {
    loadItems: (query) => new Promise((resolve) => { pending.push({query, resolve}) }),
  });
  let submitted = false;
  form.addEventListener('submit', (e) => { e.preventDefault(); submitted = true });
  const item = (value: string) => createElementFromAttrs('div', {class: 'dropdown-item', 'data-value': value}, value);

  await userEvent.click(dropdown.trigger);
  await userEvent.keyboard('a{Enter}');
  await vi.waitFor(() => expect(pending.at(-1)!.query).toBe('a'));
  expect(dropdown.values()).toEqual([]);
  pending.at(-1)!.resolve([item('a1'), item('a2')]);
  pending[0].resolve([item('stale')]);
  await vi.waitFor(() => expect(dropdown.items().map((el) => el.textContent)).toEqual(['a1', 'a2']));
  await userEvent.keyboard('{Enter}');
  expect(submitted).toBe(false);
  await userEvent.click(dropdown.findItem('a2')!);
  expect(dropdown.values()).toEqual(['a1', 'a2']);
  expect(Array.from(dropdown.trigger.querySelectorAll('.ui.label'), (el) => el.textContent)).toEqual(['a1', 'a2']);
  await expect.element(dropdown.trigger.querySelector<HTMLButtonElement>('.ui.label[data-value="a1"] .delete')).toHaveAccessibleName('Remove item "a1"');
  await userEvent.keyboard('{Backspace}');
  expect(dropdown.values()).toEqual(['a1']);
  await userEvent.keyboard('{Shift>}{Tab}{/Shift}{Enter}');
  expect(dropdown.values()).toEqual([]);
  expect(dropdown.container.hasAttribute('data-shown')).toBe(true);
  expect(document.activeElement).toBe(dropdown.filterInput);
  form.remove();
});

test('editable autocomplete submits the typed value unless an arrow key highlighted an option', async () => {
  const {promise, resolve} = Promise.withResolvers<Element[]>();
  const loadItems = vi.fn(() => promise);
  const {form, dropdown} = createFormDropdown(`<div class="dropdown-container">
    <input class="dropdown-trigger" name="fruit" aria-label="Fruit" required>
    <div class="dropdown-popup"></div>
  </div><button type="submit" name="add" value="true">Add</button>`, {loadItems});
  const submissions: Array<[FormDataEntryValue | null, FormDataEntryValue | null]> = [];
  form.addEventListener('submit', (e) => {
    e.preventDefault();
    const data = new FormData(form, e.submitter);
    submissions.push([data.get('fruit'), data.get('add')]);
  });

  await userEvent.click(dropdown.trigger);
  await userEvent.keyboard('ap');
  await vi.waitFor(() => expect(loadItems).toHaveBeenCalledWith('ap', expect.any(AbortSignal)));
  expect(dropdown.combobox.getAttribute('aria-expanded')).toBe('false');
  resolve([createElementFromAttrs('div', {class: 'dropdown-item', 'data-value': 'apple'}, 'Apple')]);
  await vi.waitFor(() => expect(dropdown.combobox.getAttribute('aria-expanded')).toBe('true'));
  await userEvent.keyboard('{Enter}');
  expect(submissions).toEqual([['ap', 'true']]);

  const changes: string[] = [];
  form.addEventListener('change', () => { changes.push(dropdown.value) });
  await userEvent.keyboard('{ArrowDown}{Enter}');
  expect(submissions).toEqual([['ap', 'true']]);
  expect(changes).toEqual(['apple']);
  expect(dropdown.value).toBe('apple');
  expect(dropdown.combobox.getAttribute('aria-expanded')).toBe('false');
  form.remove();
});
