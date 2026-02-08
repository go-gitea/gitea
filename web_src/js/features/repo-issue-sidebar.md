A sidebar combo (dropdown+list) is like this:

```html
<div class="issue-sidebar-combo" data-selection-mode="..." data-update-url="...">
  <input class="combo-value" name="..." type="hidden" value="...">
  <div class="ui dropdown">
    <div class="menu">
      <div class="item clear-selection">clear</div>
      <div class="item" data-value="..." data-scope="...">
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
The menu items must have correct `href`, otherwise the links of synchronized (cloned) items would be wrong.

Synchronization logic:
* On page load:
  * If the dropdown menu contains checked items, there will be no synchronization.
    In this case, it's assumed that the dropdown menu is already in sync with the list.
  * If the dropdown menu doesn't contain checked items, it will use dropdown's value to mark the selected items as checked.
    And the selected (checked) items will be synchronized to the list.
* On dropdown selection change:
  * The selected items will be synchronized to the list after the dropdown is hidden

The items with the same data-scope only allow one selected at a time.

The dropdown selection could work in 2 modes:
* single: only one item could be selected, it updates immediately when the item is selected.
* multiple: multiple items could be selected, it defers the update until the dropdown is hidden.

When using "scrolling menu", the items must be in the same level,
otherwise keyboard (ArrowUp/ArrowDown/Enter) won't work.
