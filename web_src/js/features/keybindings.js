import hotkeys from 'hotkeys-js';
import $ from 'jquery';

function isBindingPressed(binding) {
  let pressed = hotkeys.getPressedKeyString();

  if (pressed.length != binding.length) {
    return false;
  }

  for (let i = 0; i < binding.length; i++) {
    if (!pressed.includes(binding[i])) {
      return false;
    }
  }

  return true;
}

function checkKeybindings() {
  $("[keybinding-shortcut]").each(function(_, element) {
    if (isBindingPressed(element.getAttribute("keybinding-shortcut").split("+"))) {
      element.click();
      element.focus();
    }
  });
}

export function initKeybindings() {
  $(window).on('keydown', (event) => {
    if (!event.originalEvent.repeat) {
      checkKeybindings();
    }
  })
  hotkeys('', function(){
    // This is needed to make hotkeys-js work
  })
}
