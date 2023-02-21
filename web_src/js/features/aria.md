**This document is used as aria/a11y reference for future developers**

# Checkbox

## Accessibility-friendly Checkbox

The ideal checkboxes should be:

```html
<label><input type="checkbox"> ... </label>
```

However, related styles aren't supported (not implemented) yet, so at the moment, almost all the checkboxes are still using Fomantic UI checkbox.

## Fomantic UI Checkbox

```html
<div class="ui checkbox">
  <input type="checkbox"> <!-- class "hidden" will be added by $.checkbox() -->
  <label>...</label>
</div>
```

Then the JS `$.checkbox()` should be called to make it work with keyboard and label-clicking, then it works like the ideal checkboxes.

There is still a problem: Fomantic UI checkbox is not friendly to screen readers, so we add IDs to all the Fomantic UI checkboxes automatically by JS.

# Dropdown

## ARIA Dropdown

There are different solutions:
* combobox + listbox + option
* menu + menuitem

At the moment, `menu + menuitem` seems to work better with Fomantic UI Dropdown, so we only use it now.

```html
<div>
  <input role="combobox" aria-haspopup="listbox" aria-expanded="false" aria-controls="the-menu-listbox" aria-activedescendant="item-id-123456">
  <ul id="the-menu-listbox" role="listbox">
    <li role="option" id="item-id-123456" aria-selected="true">
      <a tabindex="-1" href="....">....</a>
    </li>
  </ul>
</div>
```


## Fomantic UI Dropdown

```html
<!-- read-only dropdown -->
<div class="ui dropdown"> <!-- focused here, then it's not perfect to use aria-activedescendant to point to the menu item -->
  <input type="hidden" ...>
  <div class="text">Default</div>
  <div class="menu" tabindex="-1"> <!-- "transition hidden|visible" classes will be added by $.dropdown() and when the dropdown is working -->
    <div class="item active selected">Default</div>
    <div class="item">...</div>
  </div>
</div>

<!-- search input dropdown -->
<div class="ui dropdown">
  <input type="hidden" ...>
  <input class="search" autocomplete="off" tabindex="0"> <!-- focused here -->
  <div class="text"></div>
  <div class="menu" tabindex="-1"> <!-- "transition hidden|visible" classes will be added by $.dropdown() and when the dropdown is working -->
    <div class="item selected">...</div>
    <div class="item">...</div>
  </div>
</div>
```
