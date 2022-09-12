import $ from 'jquery';

let keysDown = {}

function isBindingPressed(binding) {
  if (Object.keys(keysDown).length != binding.length) {
    return false;
  }

  for (let i = 0; i < binding.length; i++) {
    if (!keysDown[binding[i]]) {
      return false;
    }
  }

  return true;
}

function checkKeybindings() {
  $("[keybinding-shortcut]").each(function(_, element) {
    if (isBindingPressed(element.getAttribute("keybinding-shortcut").split("+"))) {
      element.focus();
      element.click();
    }
  });
}

export function initKeybindings() {
  $(window).on('keydown', (event) => {
    keysDown[event.key] = true;
  })
  $(window).on('keyup', (event) => {
    checkKeybindings();
    delete keysDown[event.key];
  })
}
