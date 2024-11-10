A sidebar combo (dropdown+list) is like this:

```html
<div class="issue-sidebar-combo" data-update-url="...">
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

Also, the changed items will be syncronized to the `ui list` items.

The items with the same data-scope only allow one selected at a time.
