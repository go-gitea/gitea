import $ from 'jquery';

let ariaIdCounter = 0;

function generateAriaId() {
  return `_aria_auto_id_${ariaIdCounter++}`;
}

function attachOneDropdownAria($dropdown) {
  if ($dropdown.attr('data-aria-attached') || $dropdown.hasClass('custom')) return;
  $dropdown.attr('data-aria-attached', 1);

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

  // TODO: multiple selection is not supported yet.

  const $textSearch = $dropdown.find('input.search').eq(0);
  const $focusable = $textSearch.length ? $textSearch : $dropdown; // the primary element for focus, see comment above
  if (!$focusable.length) return;

  // There are 2 possible solutions about the role: combobox or menu.
  // The idea is that if there is an input, then it's a combobox, otherwise it's a menu.
  // Since #19861 we have prepared the "combobox" solution, but didn't get enough time to put it into practice and test before.
  const isComboBox = $dropdown.find('input').length > 0;

  const focusableRole = isComboBox ? 'combobox' : 'button';
  const listPopupRole = isComboBox ? 'listbox' : 'menu';
  const listItemRole = isComboBox ? 'option' : 'menuitem';

  // make the item has role=option/menuitem, add an id if there wasn't one yet, make items as non-focusable
  // the elements inside the dropdown menu item should not be focusable, the focus should always be on the dropdown primary element.
  function prepareMenuItem($item) {
    if (!$item.attr('id')) $item.attr('id', generateAriaId());
    $item.attr({'role': listItemRole, 'tabindex': '-1'});
    $item.find('a').attr('tabindex', '-1');
  }

  // delegate the dropdown's template function to add aria attributes.
  // the "template" functions are used for dynamic creation (eg: AJAX)
  const dropdownTemplates = {...$dropdown.dropdown('setting', 'templates')};
  const dropdownTemplatesMenuOld = dropdownTemplates.menu;
  dropdownTemplates.menu = function(response, fields, preserveHTML, className) {
    // when the dropdown menu items are loaded from AJAX requests, the items are created dynamically
    const menuItems = dropdownTemplatesMenuOld(response, fields, preserveHTML, className);
    const $wrapper = $('<div>').append(menuItems);
    const $items = $wrapper.find('> .item');
    $items.each((_, item) => prepareMenuItem($(item)));
    return $wrapper.html();
  };
  $dropdown.dropdown('setting', 'templates', dropdownTemplates);

  // use tooltip's content as aria-label if there is no aria-label
  if ($dropdown.hasClass('tooltip') && $dropdown.attr('data-content') && !$dropdown.attr('aria-label')) {
    $dropdown.attr('aria-label', $dropdown.attr('data-content'));
  }

  // prepare dropdown menu list popup
  const $menu = $dropdown.find('> .menu');
  if (!$menu.attr('id')) $menu.attr('id', generateAriaId());
  $menu.find('> .item').each((_, item) => {
    prepareMenuItem($(item));
  });
  // this role could only be changed after its content is ready, otherwise some browsers+readers (like Chrome+AppleVoice) crash
  $menu.attr('role', listPopupRole);

  // make the primary element (focusable) aria-friendly
  $focusable.attr({
    'role': $focusable.attr('role') ?? focusableRole,
    'aria-haspopup': listPopupRole,
    'aria-controls': $menu.attr('id'),
    'aria-expanded': 'false',
  });

  // when showing, it has class: ".animating.in"
  // when hiding, it has class: ".visible.animating.out"
  const isMenuVisible = () => ($menu.hasClass('visible') && !$menu.hasClass('out')) || $menu.hasClass('in');

  // update aria attributes according to current active/selected item
  const refreshAria = () => {
    const menuVisible = isMenuVisible();
    $focusable.attr('aria-expanded', menuVisible ? 'true' : 'false');

    // if there is an active item, use it (the user is navigating between items)
    // otherwise use the "selected" for combobox (for the last selected item)
    const $active = $menu.find('> .item.active, > .item.selected');
    // if the popup is visible and has an active/selected item, use its id as aria-activedescendant
    if (menuVisible) {
      $focusable.attr('aria-activedescendant', $active.attr('id'));
    } else if (!isComboBox) {
      // for menu, when the popup is hidden, no need to keep the aria-activedescendant, and clear the active/selected item
      $focusable.removeAttr('aria-activedescendant');
      $active.removeClass('active').removeClass('selected');
    }
  };

  $dropdown.on('keydown', (e) => {
    // here it must use keydown event before dropdown's keyup handler, otherwise there is no Enter event in our keyup handler
    if (e.key === 'Enter') {
      let $item = $dropdown.dropdown('get item', $dropdown.dropdown('get value'));
      if (!$item) $item = $menu.find('> .item.selected'); // when dropdown filters items by input, there is no "value", so query the "selected" item
      // if the selected item is clickable, then trigger the click event.
      // we can not click any item without check, because Fomantic code might also handle the Enter event. that would result in double click.
      if ($item && ($item.is('a') || $item.hasClass('js-aria-clickable'))) $item[0].click();
    }
  });

  // use setTimeout to run the refreshAria in next tick (to make sure the Fomantic UI code has finished its work)
  // do not return any value, jQuery has return-value related behaviors.
  // when the popup is hiding, it's better to have a small "delay", because there is a Fomantic UI animation
  // without the delay for hiding, the UI will be somewhat laggy and sometimes may get stuck in the animation.
  const deferredRefreshAria = (delay = 0) => { setTimeout(refreshAria, delay) };
  $dropdown.on('keyup', (e) => { if (e.key.startsWith('Arrow')) deferredRefreshAria(); });

  // if the dropdown has been opened by focus, do not trigger the next click event again.
  // otherwise the dropdown will be closed immediately, especially on Android with TalkBack
  // * desktop event sequence: mousedown -> focus -> mouseup -> click
  // * mobile event sequence: focus -> mousedown -> mouseup -> click
  // Fomantic may stop propagation of blur event, use capture to make sure we can still get the event
  let ignoreClickPreEvents = 0, ignoreClickPreVisible = 0;
  $dropdown[0].addEventListener('mousedown', () => {
    ignoreClickPreVisible += isMenuVisible() ? 1 : 0;
    ignoreClickPreEvents++;
  }, true);
  $dropdown[0].addEventListener('focus', () => {
    ignoreClickPreVisible += isMenuVisible() ? 1 : 0;
    ignoreClickPreEvents++;
    deferredRefreshAria();
  }, true);
  $dropdown[0].addEventListener('blur', () => {
    ignoreClickPreVisible = ignoreClickPreEvents = 0;
    deferredRefreshAria(100);
  }, true);
  $dropdown[0].addEventListener('mouseup', () => {
    setTimeout(() => {
      ignoreClickPreVisible = ignoreClickPreEvents = 0;
      deferredRefreshAria(100);
    }, 0);
  }, true);
  $dropdown[0].addEventListener('click', (e) => {
    if (isMenuVisible() &&
      ignoreClickPreVisible !== 2 && // dropdown is switch from invisible to visible
      ignoreClickPreEvents === 2 // the click event is related to mousedown+focus
    ) {
      e.stopPropagation(); // if the dropdown menu has been opened by focus, do not trigger the next click event again
    }
    ignoreClickPreEvents = ignoreClickPreVisible = 0;
  }, true);
}

export function attachDropdownAria($dropdowns) {
  $dropdowns.each((_, e) => attachOneDropdownAria($(e)));
}

export function attachCheckboxAria($checkboxes) {
  $checkboxes.checkbox();

  // Fomantic UI checkbox needs to be something like: <div class="ui checkbox"><label /><input /></div>
  // It doesn't work well with <label><input />...</label>
  // To make it work with aria, the "id"/"for" attributes are necessary, so add them automatically if missing.
  // In the future, refactor to use native checkbox directly, then this patch could be removed.
  for (const el of $checkboxes) {
    const label = el.querySelector('label');
    const input = el.querySelector('input');
    if (!label || !input || input.getAttribute('id')) continue;
    const id = generateAriaId();
    input.setAttribute('id', id);
    label.setAttribute('for', id);
  }
}
