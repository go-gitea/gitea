export function initSearch() {
  const searchBar = document.querySelector<HTMLTextAreaElement>('.search-bar');
  if (!searchBar) return;
  const belowSearchContainer = document.querySelector<HTMLTextAreaElement>('.search-predict-show');
  if (!belowSearchContainer) return;
  const clearIcon = document.querySelector<HTMLTextAreaElement>('.search-icon-clear');
  if (!clearIcon) return;
  const textArea = document.querySelector<HTMLTextAreaElement>('.search-input-area textarea.search-input');
  if (!textArea) return;
  const elForm = document.querySelector<HTMLFormElement>('.search-form form');
  if (!elForm) return;

  function onSearchTextAreaFocus(this: HTMLTextAreaElement, _ev: FocusEvent) {
    belowSearchContainer.style.display = 'block';
  }
  function onSearchTextAreaBlur(this: HTMLTextAreaElement, _ev: FocusEvent) {
    belowSearchContainer.style.display = 'none';
  }
  function onSearchTextChange() {
    // adjust the height
    textArea.style.height = 'auto';
    textArea.style.height = `${textArea.scrollHeight+8}px`;
    // display clear icon
    if (textArea.value !== '') {
      clearIcon.style.display = 'flex';
    } else {
      clearIcon.style.display = 'none';
    }
  }

  searchBar.addEventListener('click', () => {
    textArea.focus();
  });
  textArea.addEventListener('focus', onSearchTextAreaFocus);
  textArea.addEventListener('blur', onSearchTextAreaBlur);
  textArea.addEventListener('keydown', (ev: KeyboardEvent) => {
    if (ev.code === 'Enter' && !ev.shiftKey) {
      elForm.submit();
      ev.preventDefault(); // Prevents the addition of a new line in the text field
    }
  })
  clearIcon.addEventListener('click', () => {
    textArea.value = '';
    textArea.focus();
    clearIcon.style.display = 'none';
    textArea.style.height = '46px';
  });
  if (textArea.addEventListener) {
    textArea.addEventListener('input', () => {
      // event handling code for sane browsers
      onSearchTextChange();
    }, false);
  } else if (textArea.attachEvent) {
    textArea.attachEvent('onpropertychange', () => {
      // IE-specific event handling code
      onSearchTextChange();
    });
  }
}
