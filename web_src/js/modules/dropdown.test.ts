import {userEvent} from 'vitest/browser';
import {createElementFromAttrs, createElementFromHTML} from '../utils/dom.ts';
import {closestDropdown, hideScopedEmptyDividers, initDropdown} from './dropdown.ts';

function createDropdown(html: string, opts?: Parameters<typeof initDropdown>[1]) {
  const container = createElementFromHTML<HTMLElement>(html);
  document.body.append(container);
  return initDropdown(container, opts);
}

test('select in a field is named by its label, filters by keyboard and fires change only on a new value', async () => {
  const field = createElementFromHTML(`<div class="field"><label>Fruit</label><div class="dropdown-container" data-kind="select">
    <button type="button" class="dropdown-trigger"><span class="dropdown-text" data-placeholder="Pick">Apple</span></button>
    <input type="hidden" class="dropdown-value" value="apple">
    <div class="dropdown-popup">
      <input class="dropdown-filter">
      <div class="dropdown-item" data-value="">Clear</div>
      <div class="dropdown-item" data-value="apple">Apple</div>
      <div class="dropdown-item" data-value="banana" data-text="Banana!"><b>Banana</b></div>
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
  await userEvent.keyboard('ban{Enter}');
  expect(changes).toEqual(['banana']);
  expect(dropdown.trigger.textContent).toBe('Banana!');
  expect(dropdown.isShown()).toBe(false);
  expect(document.activeElement).toBe(dropdown.trigger);
  expect(dropdown.container.lastElementChild).toBe(dropdown.popup);
  expect(dropdown.items().some((item) => item.hidden)).toBe(false);

  await userEvent.keyboard('{ArrowDown}{Enter}');
  await userEvent.click(dropdown.trigger);
  await userEvent.click(dropdown.findItem('banana')!);
  dropdown.setValue('apple');
  expect(changes).toEqual(['banana']);
  expect(dropdown.trigger.textContent).toBe('Apple');
  dropdown.setValue('');
  expect(dropdown.trigger.textContent).toBe('Pick');
  field.remove();
});

test('menu navigates by keyboard, keeps popup controls in their form and closes before item handlers, on Escape, Tab and removal', async () => {
  const dropdown = createDropdown(`<div class="dropdown-container">
    <button type="button" class="dropdown-trigger">Menu</button>
    <div class="dropdown-popup">
      <input type="hidden" name="mode" value="all">
      <button type="button" class="dropdown-item">Item</button>
      <div class="dropdown-item" aria-disabled="true">Disabled</div>
      <button type="button" class="dropdown-item">Item two</button>
    </div>
  </div>`);
  const form = document.createElement('form');
  document.body.append(form);
  form.append(dropdown.container);
  const [item, disabledItem, otherItem] = dropdown.items();
  let itemContainer: Element | null = null;
  item.addEventListener('click', () => { itemContainer = item.closest('.dropdown-container') });

  await userEvent.click(dropdown.trigger);
  expect(form.contains(dropdown.popup)).toBe(false);
  expect(new FormData(form).get('mode')).toBe('all');
  expect(closestDropdown(item)).toBe(dropdown);
  disabledItem.click();
  expect(dropdown.isShown()).toBe(true);
  await userEvent.click(item);
  expect(itemContainer).toBe(dropdown.container);
  expect(dropdown.isShown()).toBe(false);

  await userEvent.keyboard('{ArrowDown}');
  expect(dropdown.highlighted).toBe(item);
  await userEvent.keyboard('{ArrowDown}');
  expect(dropdown.highlighted).toBe(otherItem);
  await userEvent.keyboard('{PageUp}');
  expect(dropdown.highlighted).toBe(item);
  await userEvent.keyboard('{PageDown}');
  expect(dropdown.highlighted).toBe(otherItem);
  await userEvent.keyboard('i');
  expect(dropdown.highlighted).toBe(item);
  await userEvent.keyboard('i');
  expect(dropdown.highlighted).toBe(otherItem);
  await userEvent.keyboard('{Escape}');
  expect(dropdown.isShown()).toBe(false);
  expect(document.activeElement).toBe(dropdown.trigger);
  await userEvent.keyboard('{ArrowUp}');
  dropdown.popup.append(createElementFromHTML('<button type="button" class="dropdown-item">Late</button>'));
  await userEvent.keyboard('{Tab}');
  expect(dropdown.isShown()).toBe(false);
  await userEvent.click(dropdown.trigger);
  form.remove();
  await vi.waitFor(() => expect(dropdown.isShown()).toBe(false));
  expect(dropdown.popup.isConnected).toBe(false);
});

test('remote items apply the latest query only', async () => {
  const pending: Array<{query: string, resolve: (items: Element[]) => void}> = [];
  const dropdown = createDropdown(`<div class="dropdown-container" data-kind="multiselect">
    <button type="button" class="dropdown-trigger"><span class="dropdown-text"></span></button>
    <input type="hidden" class="dropdown-value" value="">
    <div class="dropdown-popup">
      <input class="dropdown-filter">
      <div class="dropdown-item" data-value="static" data-static>Static</div>
    </div>
  </div>`, {
    loadItems: (query) => new Promise((resolve) => { pending.push({query, resolve}) }),
  });
  const item = (value: string) => createElementFromAttrs('div', {class: 'dropdown-item', 'data-value': value}, value);

  await userEvent.click(dropdown.trigger);
  await userEvent.keyboard('a');
  await vi.waitFor(() => expect(pending.at(-1)!.query).toBe('a'));
  pending.at(-1)!.resolve([item('a1'), item('a2')]);
  pending[0].resolve([item('stale')]);
  await vi.waitFor(() => expect(dropdown.items().map((el) => el.textContent)).toEqual(['Static', 'a1', 'a2']));
  await userEvent.click(dropdown.findItem('a2')!);
  await userEvent.click(dropdown.findItem('static')!);
  expect(dropdown.values()).toEqual(['a2', 'static']);
  expect(dropdown.trigger.textContent).toBe('a2, Static');
  await userEvent.click(dropdown.findItem('a2')!);
  expect(dropdown.values()).toEqual(['static']);
  expect(dropdown.isShown()).toBe(true);
  dropdown.container.remove();
});

test('hideScopedEmptyDividers', () => {
  const container = createElementFromHTML(`<div>
<div class="dropdown-divider"></div>
<div class="dropdown-item">a</div>
<div class="dropdown-divider"></div>
<div class="dropdown-divider"></div>
<div class="dropdown-item" data-scope="b">b</div>
<div class="dropdown-divider" data-scope="c"></div>
<div class="dropdown-item" data-scope="c" hidden="">c</div>
<div class="dropdown-divider" data-scope=""></div>
<div class="dropdown-item tw-hidden">d</div>
<div class="dropdown-divider"></div>
</div>`);
  hideScopedEmptyDividers(container);
  expect(container.innerHTML).toEqual(`
<div class="dropdown-divider" hidden=""></div>
<div class="dropdown-item">a</div>
<div class="dropdown-divider" hidden=""></div>
<div class="dropdown-divider"></div>
<div class="dropdown-item" data-scope="b">b</div>
<div class="dropdown-divider" data-scope="c" hidden=""></div>
<div class="dropdown-item" data-scope="c" hidden="">c</div>
<div class="dropdown-divider" data-scope="" hidden=""></div>
<div class="dropdown-item tw-hidden">d</div>
<div class="dropdown-divider" hidden=""></div>
`);
});
