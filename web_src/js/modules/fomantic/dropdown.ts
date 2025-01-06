import $ from 'jquery';
import {generateAriaId} from './base.ts';
import type {FomanticInitFunction} from '../../types.ts';
import {queryElems} from '../../utils/dom.ts';

const ariaPatchKey = '_giteaAriaPatchDropdown';
const fomanticDropdownFn = $.fn.dropdown;

// use our own `$().dropdown` function to patch Fomantic's dropdown module
export function initAriaDropdownPatch() {
  if ($.fn.dropdown === ariaDropdownFn) throw new Error('initAriaDropdownPatch could only be called once');
  $.fn.dropdown = ariaDropdownFn;
  $.fn.fomanticExt.onResponseKeepSelectedItem = onResponseKeepSelectedItem;
  (ariaDropdownFn as FomanticInitFunction).settings = fomanticDropdownFn.settings;
}

// the patched `$.fn.dropdown` function, it passes the arguments to Fomantic's `$.fn.dropdown` function, and:
// * it does the one-time attaching on the first call
// * it delegates the `onLabelCreate` to the patched `onLabelCreate` to add necessary aria attributes
function ariaDropdownFn(...args: Parameters<FomanticInitFunction>) {
  const ret = fomanticDropdownFn.apply(this, args);

  // if the `$().dropdown()` call is without arguments, or it has non-string (object) argument,
  // it means that this call will reset the dropdown internal settings, then we need to re-delegate the callbacks.
  const needDelegate = (!args.length || typeof args[0] !== 'string');
  for (const el of this) {
    if (!el[ariaPatchKey]) {
      attachInit(el);
    }
    if (needDelegate) {
      delegateOne($(el));
    }
  }
  return ret;
}

// make the item has role=option/menuitem, add an id if there wasn't one yet, make items as non-focusable
// the elements inside the dropdown menu item should not be focusable, the focus should always be on the dropdown primary element.
function updateMenuItem(dropdown: HTMLElement, item: HTMLElement) {
  if (!item.id) item.id = generateAriaId();
  item.setAttribute('role', dropdown[ariaPatchKey].listItemRole);
  item.setAttribute('tabindex', '-1');
  for (const el of item.querySelectorAll('a, input, button')) el.setAttribute('tabindex', '-1');
}
/**
 * make the label item and its "delete icon" have correct aria attributes
 * @param {HTMLElement} label
 */
function updateSelectionLabel(label: HTMLElement) {
  // the "label" is like this: "<a|div class="ui label" data-value="1">the-label-name <i|svg class="delete icon"/></a>"
  if (!label.id) {
    label.id = generateAriaId();
  }
  label.tabIndex = -1;

  const deleteIcon = label.querySelector('.delete.icon');
  if (deleteIcon) {
    deleteIcon.setAttribute('aria-hidden', 'false');
    deleteIcon.setAttribute('aria-label', window.config.i18n.remove_label_str.replace('%s', label.getAttribute('data-value')));
    deleteIcon.setAttribute('role', 'button');
  }
}

function processMenuItems($dropdown, dropdownCall) {
  const hideEmptyDividers = dropdownCall('setting', 'hideDividers') === 'empty';
  const itemsMenu = $dropdown[0].querySelector('.scrolling.menu') || $dropdown[0].querySelector('.menu');
  if (hideEmptyDividers) hideScopedEmptyDividers(itemsMenu);
}

// delegate the dropdown's template functions and callback functions to add aria attributes.
function delegateOne($dropdown: any) {
  const dropdownCall = fomanticDropdownFn.bind($dropdown);

  // If there is a "search input" in the "menu", Fomantic will only "focus the input" but not "toggle the menu" when the "dropdown icon" is clicked.
  // Actually, Fomantic UI doesn't support such layout/usage. It needs to patch the "focusSearch" / "blurSearch" functions to make sure it toggles the menu.
  const oldFocusSearch = dropdownCall('internal', 'focusSearch');
  const oldBlurSearch = dropdownCall('internal', 'blurSearch');
  // * If the "dropdown icon" is clicked, Fomantic calls "focusSearch", so show the menu
  dropdownCall('internal', 'focusSearch', function () { dropdownCall('show'); oldFocusSearch.call(this) });
  // * If the "dropdown icon" is clicked again when the menu is visible, Fomantic calls "blurSearch", so hide the menu
  dropdownCall('internal', 'blurSearch', function () { oldBlurSearch.call(this); dropdownCall('hide') });

  const oldFilterItems = dropdownCall('internal', 'filterItems');
  dropdownCall('internal', 'filterItems', function (...args: any[]) {
    oldFilterItems.call(this, ...args);
    processMenuItems($dropdown, dropdownCall);
  });

  const oldShow = dropdownCall('internal', 'show');
  dropdownCall('internal', 'show', function (...args: any[]) {
    oldShow.call(this, ...args);
    processMenuItems($dropdown, dropdownCall);
  });

  // the "template" functions are used for dynamic creation (eg: AJAX)
  const dropdownTemplates = {...dropdownCall('setting', 'templates'), t: performance.now()};
  const dropdownTemplatesMenuOld = dropdownTemplates.menu;
  dropdownTemplates.menu = function(response: any, fields: any, preserveHTML: any, className: Record<string, string>) {
    // when the dropdown menu items are loaded from AJAX requests, the items are created dynamically
    const menuItems = dropdownTemplatesMenuOld(response, fields, preserveHTML, className);
    const div = document.createElement('div');
    div.innerHTML = menuItems;
    const $wrapper = $(div);
    const $items = $wrapper.find('> .item');
    $items.each((_, item) => updateMenuItem($dropdown[0], item));
    $dropdown[0][ariaPatchKey].deferredRefreshAriaActiveItem();
    return $wrapper.html();
  };
  dropdownCall('setting', 'templates', dropdownTemplates);

  // the `onLabelCreate` is used to add necessary aria attributes for dynamically created selection labels
  const dropdownOnLabelCreateOld = dropdownCall('setting', 'onLabelCreate');
  dropdownCall('setting', 'onLabelCreate', function(value: any, text: string) {
    const $label = dropdownOnLabelCreateOld.call(this, value, text);
    updateSelectionLabel($label[0]);
    return $label;
  });

  const oldSet = dropdownCall('internal', 'set');
  const oldSetDirection = oldSet.direction;
  oldSet.direction = function($menu: any) {
    oldSetDirection.call(this, $menu);
    const classNames = dropdownCall('setting', 'className');
    $menu = $menu || $dropdown.find('> .menu');
    const elMenu = $menu[0];
    // detect whether the menu is outside the viewport, and adjust the position
    // there is a bug in fomantic's builtin `direction` function, in some cases (when the menu width is only a little larger) it wrongly opens the menu at right and triggers the scrollbar.
    elMenu.classList.add(classNames.loading);
    if (elMenu.getBoundingClientRect().right > document.documentElement.clientWidth) {
      elMenu.classList.add(classNames.leftward);
    }
    elMenu.classList.remove(classNames.loading);
  };
}

// for static dropdown elements (generated by server-side template), prepare them with necessary aria attributes
function attachStaticElements(dropdown: HTMLElement, focusable: HTMLElement, menu: HTMLElement) {
  // prepare static dropdown menu list popup
  if (!menu.id) {
    menu.id = generateAriaId();
  }

  $(menu).find('> .item').each((_, item) => updateMenuItem(dropdown, item));

  // this role could only be changed after its content is ready, otherwise some browsers+readers (like Chrome+AppleVoice) crash
  menu.setAttribute('role', dropdown[ariaPatchKey].listPopupRole);

  // prepare selection label items
  for (const label of dropdown.querySelectorAll<HTMLElement>('.ui.label')) {
    updateSelectionLabel(label);
  }

  // make the primary element (focusable) aria-friendly
  focusable.setAttribute('role', focusable.getAttribute('role') ?? dropdown[ariaPatchKey].focusableRole);
  focusable.setAttribute('aria-haspopup', dropdown[ariaPatchKey].listPopupRole);
  focusable.setAttribute('aria-controls', menu.id);
  focusable.setAttribute('aria-expanded', 'false');

  // use tooltip's content as aria-label if there is no aria-label
  const tooltipContent = dropdown.getAttribute('data-tooltip-content');
  if (tooltipContent && !dropdown.getAttribute('aria-label')) {
    dropdown.setAttribute('aria-label', tooltipContent);
  }
}

function attachInit(dropdown: HTMLElement) {
  dropdown[ariaPatchKey] = {};
  if (dropdown.classList.contains('custom')) return;

  // Dropdown has 2 different focusing behaviors
  // * with search input: the input is focused, and it works with aria-activedescendant pointing another sibling element.
  // * without search input (but the readonly text), the dropdown itself is focused. then the aria-activedescendant points to the element inside dropdown
  // Some desktop screen readers may change the focus, but dropdown requires that the focus must be on its primary element, then they don't work well.

  // Expected user interactions for dropdown with aria support:
  // * user can use Tab to focus in the dropdown, then the dropdown menu (list) will be shown
  // * user presses Tab on the focused dropdown to move focus to next sibling focusable element (but not the menu item)
  // * user can use arrow key Up/Down to navigate between menu items
  // * when user presses Enter:
  //    - if the menu item is clickable (eg: <a>), then trigger the click event
  //    - otherwise, the dropdown control (low-level code) handles the Enter event, hides the dropdown menu

  // TODO: multiple selection is only partially supported. Check and test them one by one in the future.

  const textSearch = dropdown.querySelector<HTMLElement>('input.search');
  const focusable = textSearch || dropdown; // the primary element for focus, see comment above
  if (!focusable) return;

  // as a combobox, the input should not have autocomplete by default
  if (textSearch && !textSearch.getAttribute('autocomplete')) {
    textSearch.setAttribute('autocomplete', 'off');
  }

  let menu = $(dropdown).find('> .menu')[0];
  if (!menu) {
    // some "multiple selection" dropdowns don't have a static menu element in HTML, we need to pre-create it to make it have correct aria attributes
    menu = document.createElement('div');
    menu.classList.add('menu');
    dropdown.append(menu);
  }

  // There are 2 possible solutions about the role: combobox or menu.
  // The idea is that if there is an input, then it's a combobox, otherwise it's a menu.
  // Since #19861 we have prepared the "combobox" solution, but didn't get enough time to put it into practice and test before.
  const isComboBox = dropdown.querySelectorAll('input').length > 0;

  dropdown[ariaPatchKey].focusableRole = isComboBox ? 'combobox' : 'menu';
  dropdown[ariaPatchKey].listPopupRole = isComboBox ? 'listbox' : '';
  dropdown[ariaPatchKey].listItemRole = isComboBox ? 'option' : 'menuitem';

  attachDomEvents(dropdown, focusable, menu);
  attachStaticElements(dropdown, focusable, menu);
}

function attachDomEvents(dropdown: HTMLElement, focusable: HTMLElement, menu: HTMLElement) {
  // when showing, it has class: ".animating.in"
  // when hiding, it has class: ".visible.animating.out"
  const isMenuVisible = () => (menu.classList.contains('visible') && !menu.classList.contains('out')) || menu.classList.contains('in');

  // update aria attributes according to current active/selected item
  const refreshAriaActiveItem = () => {
    const menuVisible = isMenuVisible();
    focusable.setAttribute('aria-expanded', menuVisible ? 'true' : 'false');

    // if there is an active item, use it (the user is navigating between items)
    // otherwise use the "selected" for combobox (for the last selected item)
    const active = $(menu).find('> .item.active, > .item.selected')[0];
    if (!active) return;
    // if the popup is visible and has an active/selected item, use its id as aria-activedescendant
    if (menuVisible) {
      focusable.setAttribute('aria-activedescendant', active.id);
    } else if (dropdown[ariaPatchKey].listPopupRole === 'menu') {
      // for menu, when the popup is hidden, no need to keep the aria-activedescendant, and clear the active/selected item
      focusable.removeAttribute('aria-activedescendant');
      active.classList.remove('active', 'selected');
    }
  };

  dropdown.addEventListener('keydown', (e: KeyboardEvent) => {
    // here it must use keydown event before dropdown's keyup handler, otherwise there is no Enter event in our keyup handler
    if (e.key === 'Enter') {
      const dropdownCall = fomanticDropdownFn.bind($(dropdown));
      let $item = dropdownCall('get item', dropdownCall('get value'));
      if (!$item) $item = $(menu).find('> .item.selected'); // when dropdown filters items by input, there is no "value", so query the "selected" item
      // if the selected item is clickable, then trigger the click event.
      // we can not click any item without check, because Fomantic code might also handle the Enter event. that would result in double click.
      if ($item?.[0]?.matches('a, .js-aria-clickable')) $item[0].click();
    }
  });

  // use setTimeout to run the refreshAria in next tick (to make sure the Fomantic UI code has finished its work)
  // do not return any value, jQuery has return-value related behaviors.
  // when the popup is hiding, it's better to have a small "delay", because there is a Fomantic UI animation
  // without the delay for hiding, the UI will be somewhat laggy and sometimes may get stuck in the animation.
  const deferredRefreshAriaActiveItem = (delay = 0) => { setTimeout(refreshAriaActiveItem, delay) };
  dropdown[ariaPatchKey].deferredRefreshAriaActiveItem = deferredRefreshAriaActiveItem;
  dropdown.addEventListener('keyup', (e) => { if (e.key.startsWith('Arrow')) deferredRefreshAriaActiveItem(); });

  // if the dropdown has been opened by focus, do not trigger the next click event again.
  // otherwise the dropdown will be closed immediately, especially on Android with TalkBack
  // * desktop event sequence: mousedown -> focus -> mouseup -> click
  // * mobile event sequence: focus -> mousedown -> mouseup -> click
  // Fomantic may stop propagation of blur event, use capture to make sure we can still get the event
  let ignoreClickPreEvents = 0, ignoreClickPreVisible = 0;
  dropdown.addEventListener('mousedown', () => {
    ignoreClickPreVisible += isMenuVisible() ? 1 : 0;
    ignoreClickPreEvents++;
  }, true);
  dropdown.addEventListener('focus', () => {
    ignoreClickPreVisible += isMenuVisible() ? 1 : 0;
    ignoreClickPreEvents++;
    deferredRefreshAriaActiveItem();
  }, true);
  dropdown.addEventListener('blur', () => {
    ignoreClickPreVisible = ignoreClickPreEvents = 0;
    deferredRefreshAriaActiveItem(100);
  }, true);
  dropdown.addEventListener('mouseup', () => {
    setTimeout(() => {
      ignoreClickPreVisible = ignoreClickPreEvents = 0;
      deferredRefreshAriaActiveItem(100);
    }, 0);
  }, true);
  dropdown.addEventListener('click', (e: MouseEvent) => {
    if (isMenuVisible() &&
      ignoreClickPreVisible !== 2 && // dropdown is switch from invisible to visible
      ignoreClickPreEvents === 2 // the click event is related to mousedown+focus
    ) {
      e.stopPropagation(); // if the dropdown menu has been opened by focus, do not trigger the next click event again
    }
    ignoreClickPreEvents = ignoreClickPreVisible = 0;
  }, true);
}

// Although Fomantic Dropdown supports "hideDividers", it doesn't really work with our "scoped dividers"
// At the moment, "label dropdown items" use scopes, a sample case is:
// * a-label
// * divider
// * scope/1
// * scope/2
// * divider
// * z-label
// when the "scope/*" are filtered out, we'd like to see "a-label" and "z-label" without the divider.
export function hideScopedEmptyDividers(container: Element) {
  const visibleItems: Element[] = [];
  const curScopeVisibleItems: Element[] = [];
  let curScope: string = '', lastVisibleScope: string = '';
  const isScopedDivider = (item: Element) => item.matches('.divider') && item.hasAttribute('data-scope');
  const hideDivider = (item: Element) => item.classList.add('hidden', 'transition'); // dropdown has its own classes to hide items

  const handleScopeSwitch = (itemScope: string) => {
    if (curScopeVisibleItems.length === 1 && isScopedDivider(curScopeVisibleItems[0])) {
      hideDivider(curScopeVisibleItems[0]);
    } else if (curScopeVisibleItems.length) {
      if (isScopedDivider(curScopeVisibleItems[0]) && lastVisibleScope === curScope) {
        hideDivider(curScopeVisibleItems[0]);
        curScopeVisibleItems.shift();
      }
      visibleItems.push(...curScopeVisibleItems);
      lastVisibleScope = curScope;
    }
    curScope = itemScope;
    curScopeVisibleItems.length = 0;
  };

  // hide the scope dividers if the scope items are empty
  for (const item of container.children) {
    const itemScope = item.getAttribute('data-scope') || '';
    if (itemScope !== curScope) {
      handleScopeSwitch(itemScope);
    }
    if (!item.classList.contains('filtered') && !item.classList.contains('tw-hidden')) {
      curScopeVisibleItems.push(item as HTMLElement);
    }
  }
  handleScopeSwitch('');

  // hide all leading and trailing dividers
  while (visibleItems.length) {
    if (!visibleItems[0].matches('.divider')) break;
    hideDivider(visibleItems[0]);
    visibleItems.shift();
  }
  while (visibleItems.length) {
    if (!visibleItems[visibleItems.length - 1].matches('.divider')) break;
    hideDivider(visibleItems[visibleItems.length - 1]);
    visibleItems.pop();
  }
  // hide all duplicate dividers, hide current divider if next sibling is still divider
  // no need to update "visibleItems" array since this is the last loop
  for (const item of visibleItems) {
    if (!item.matches('.divider')) continue;
    if (item.nextElementSibling?.matches('.divider')) hideDivider(item);
  }
}

function onResponseKeepSelectedItem(dropdown: typeof $|HTMLElement, selectedValue: string) {
  // There is a bug in fomantic dropdown when using "apiSettings" to fetch data
  // * when there is a selected item, the dropdown insists on hiding the selected one from the list:
  // * in the "filter" function: ('[data-value="'+value+'"]').addClass(className.filtered)
  //
  // When user selects one item, and click the dropdown again,
  // then the dropdown only shows other items and will select another (wrong) one.
  // It can't be easily fix by using setTimeout(patch, 0) in `onResponse` because the `onResponse` is called before another `setTimeout(..., timeLeft)`
  // Fortunately, the "timeLeft" is controlled by "loadingDuration" which is always zero at the moment, so we can use `setTimeout(..., 10)`
  const elDropdown = (dropdown instanceof HTMLElement) ? dropdown : dropdown[0];
  setTimeout(() => {
    queryElems(elDropdown, `.menu .item[data-value="${CSS.escape(selectedValue)}"].filtered`, (el) => el.classList.remove('filtered'));
    $(elDropdown).dropdown('set selected', selectedValue ?? '');
  }, 10);
}
