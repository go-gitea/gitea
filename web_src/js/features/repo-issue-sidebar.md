A sidebar combo (dropdown+list) is like this:

```html
<div class="issue-sidebar-combo" data-selection-mode="..." data-update-url="...">
  <input class="combo-value" name="..." type="hidden" value="...">
  <div class="dropdown-container" data-keep-open>
    <button type="button" class="dropdown-trigger btn">Select items</button>
    <div class="dropdown-popup issue-sidebar-popup">
      <input class="dropdown-filter" type="search" placeholder="Filter items">
      <div class="dropdown-item item clear-selection">clear</div>
      <div class="dropdown-item item" data-value="..." data-scope="...">
        <span class="item-check-mark">...</span>
        ...
      </div>
    </div>
  </div>
  <div class="ui list">
    <span class="item empty-list">no item</span>
    <span class="item">...</span>
  </div>
</div>
```

When the selected items change, the `combo-value` input will be updated.
If there is `data-update-url`, it also calls backend to attach/detach the changed items.

Also, the changed items will be synchronized to the `ui list` items.
The popup items must have correct `href`, otherwise the links of synchronized (cloned) items would be wrong.
The controller removes dropdown classes and accessibility attributes from the cloned list items.

The `ui list` is optional, so a single dropdown can also work, to select items and update them to backend.

Synchronization logic:
* On page load:
  * If the dropdown menu contains checked items, there will be no synchronization.
    In this case, it's assumed that the dropdown menu is already in sync with the list.
  * If the dropdown menu doesn't contain checked items, it will use dropdown's value to mark the selected items as checked.
    And the selected (checked) items will be synchronized to the list.
    Dropdown's value should be empty if the there is no dropdown item but a pre-defined list item need to be displayed.
* On dropdown selection change:
  * The selected items will be synchronized to the list on `ce-dropdown-hide`.

The items with the same data-scope only allow one selected at a time.

The dropdown selection could work in 2 modes:
* single: only one item could be selected, it updates immediately when the item is selected.
* multiple: multiple items could be selected, it defers the update until the dropdown is hidden.

The controller binds item clicks on `dropdown.popup` because the popup moves into `body` while open.
Multiple-selection dropdowns use `data-keep-open` and maintain `.checked` items, including exclusive scopes.
Single-selection dropdowns omit `data-keep-open` and update as soon as an item is selected.
Items with `data-can-change="false"` retain their selection when clicked.
