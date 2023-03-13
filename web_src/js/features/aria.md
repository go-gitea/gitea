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

There are 2 possible solutions about the role: combobox or menu

1. Detect if the dropdown has an input, if yes, it works like a combobox, otherwise it works like a menu
2. Always use "combobox", never use "menu"

According to the public discussion with fsologureng in chat channel, we think it's better to use "combobox" for all dropdowns.

> On the old web there were many menus implemented with an auto-submit select,
> but that didn't change the fact that they were selects for screen readers.
> That is the case with Fomantic dropdowns as used in Gitea.
> Implementations of auto-submit select menus fell behind in modern web design precisely because they are not usable or accessible."
>
> We can mark all "dropdown" as "combobox", never use "menu" in code. Do you think this solution is clear enough?
>
> Yes. I think it will provide better accessibility because is more coherent with the current fomantic based implementation.

Reference:

* Combobox:
  * https://www.w3.org/WAI/ARIA/apg/patterns/combobox/
  * A combobox is an **input widget** with an associated popup that enables users to select a value for the combobox from
    a collection of possible values. In some implementations, the popup presents allowed values, while in other implementations,
    the popup presents suggested values, and users may either select one of the suggestions or type a value.
* Menu:
  * https://www.w3.org/WAI/ARIA/apg/patterns/menubar/
  * A menu is a widget that offers a list of choices to the user, such as a set of **actions or functions**.

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
