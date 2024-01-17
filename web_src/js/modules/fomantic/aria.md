# Background

This document is used as aria/accessibility(a11y) reference for future developers.

There are a lot of a11y problems in the Fomantic UI library. This `aria.js` is used
as a workaround to make the UI more accessible.

The `aria.js` is designed to avoid touching the official Fomantic UI library,
and to be as independent as possible, so it can be easily modified/removed in the future.

To test the aria/accessibility with screen readers, developers can use the following steps:

* On macOS, you can use VoiceOver.
  * Press `Command + F5` to turn on VoiceOver.
  * Try to operate the UI with keyboard-only.
  * Use Tab/Shift+Tab to switch focus between elements.
  * Arrow keys to navigate between menu/combobox items (only aria-active, not really focused).
  * Press Enter to trigger the aria-active element.
* On Android, you can use TalkBack.
  * Go to Settings -> Accessibility -> TalkBack, turn it on.
  * Long-press or press+swipe to switch the aria-active element (not really focused).
  * Double-tap means old single-tap on the aria-active element.
  * Double-finger swipe means old single-finger swipe.
* TODO: on Windows, on Linux, on iOS

# Known Problems

* Tested with Apple VoiceOver: If a dropdown menu/combobox is opened by mouse click, then arrow keys don't work.
  But if the dropdown is opened by keyboard Tab, then arrow keys work, and from then on, the keys almost work with mouse click too.
  The clue: when the dropdown is only opened by mouse click, VoiceOver doesn't send 'keydown' events of arrow keys to the DOM,
  VoiceOver expects to use arrow keys to navigate between some elements, but it couldn't.
  Users could use Option+ArrowKeys to navigate between menu/combobox items or selection labels if the menu/combobox is opened by mouse click.

# Checkbox

## Accessibility-friendly Checkbox

The ideal checkboxes should be:

```html
<label><input type="checkbox"> ... </label>
```

However, related CSS styles aren't supported (not implemented) yet, so at the moment,
almost all the checkboxes are still using Fomantic UI checkbox.

## Fomantic UI Checkbox

```html
<div class="ui checkbox">
  <input type="checkbox"> <!-- class "hidden" will be added by $.checkbox() -->
  <label>...</label>
</div>
```

Then the JS `$.checkbox()` should be called to make it work with keyboard and label-clicking,
then it works like the ideal checkboxes.

There is still a problem: Fomantic UI checkbox is not friendly to screen readers,
so we add IDs to all the Fomantic UI checkboxes automatically by JS.
If the `label` part is empty, then the checkbox needs to get the `aria-label` attribute manually.

# Fomantic Dropdown

Fomantic Dropdown is designed to be used for many purposes:

* Menu (the profile menu in navbar, the language menu in footer)
* Popup (the branch/tag panel, the review box)
* Simple `<select>` , used in many forms
* Searchable option-list with static items (used in many forms)
* Searchable option-list with dynamic items (ajax)
* Searchable multiple selection option-list with dynamic items: the repo topic setting
* More complex usages, like the Issue Label selector

Fomantic Dropdown requires that the focus must be on its primary element.
If the focus changes, it hides or panics.

At the moment, `aria.js` only tries to partially resolve the a11y problems for dropdowns with items.

There are different solutions:

* combobox + listbox + option:
  * https://www.w3.org/WAI/ARIA/apg/patterns/combobox/
  * A combobox is an input widget with an associated popup that enables users to select a value for the combobox from
    a collection of possible values. In some implementations, the popup presents allowed values, while in other implementations,
    the popup presents suggested values, and users may either select one of the suggestions or type a value.
* menu + menuitem:
  * https://www.w3.org/WAI/ARIA/apg/patterns/menubar/
  * A menu is a widget that offers a list of choices to the user, such as a set of actions or functions.

The current approach is: detect if the dropdown has an input,
if yes, it works like a combobox, otherwise it works like a menu.
Multiple selection dropdown is not well-supported yet, it needs more work.

Some important pages for dropdown testing:

* Home(dashboard) page, the "Create Repo" / "Profile" / "Language" menu.
* Create New Repo page, a lot of dropdowns as combobox.
* Collaborators page, the "permission" dropdown (the old behavior was not quite good, it just works).

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
