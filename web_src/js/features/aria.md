**This document is used as aria/a11y reference for future developers**

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
  <div class="menu transition hidden" tabindex="-1">
    <div class="item active selected">Default</div>
    <div class="item">...</div>
  </div>
</div>

<!-- search input dropdown -->
<div class="ui dropdown">
  <input type="hidden" ...>
  <input class="search" autocomplete="off" tabindex="0"> <!-- focused here -->
  <div class="text"></div>
  <div class="menu transition visible" tabindex="-1">
    <div class="item selected">...</div>
    <div class="item">...</div>
  </div>
</div>
```
