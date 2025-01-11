import {hideElem, showElem} from './utils/dom.ts';

export function initSearch() {
  const searchBar = document.querySelector<HTMLElement>('.search-bar');
  if (!searchBar) return;
  const belowSearchContainer = document.querySelector<HTMLElement>('.search-predict-show');
  if (!belowSearchContainer) return;
  const clearIcon = document.querySelector<HTMLElement>('.search-icon-clear');
  if (!clearIcon) return;
  const searchInputArea = document.querySelector<HTMLElement>('.search-input-area');
  if (!searchInputArea) return;
  const textArea = document.querySelector<HTMLTextAreaElement>('.search-input-area textarea.search-input');
  if (!textArea) return;
  const elForm = document.querySelector<HTMLFormElement>('.search-form form');
  if (!elForm) return;

  function onSearchTextAreaFocus(this: HTMLTextAreaElement, _ev: FocusEvent) {
    showElem(belowSearchContainer);
    searchBar.style.borderRadius = '24px 24px 0 0';
    searchBar.style.background = 'var(--color-search-bar-background-hover)';
    searchBar.style.boxShadow = 'var(--shadow-search-box-hover)';
    searchBar.style.borderColor = 'transparent';
    //TODO: add dark mode focus darkening
  }
  function onSearchTextAreaBlur(this: HTMLTextAreaElement, _ev: FocusEvent) {
    hideElem(belowSearchContainer);
    searchBar.style.background = 'var(--color-search-bar-background)';
    searchBar.style.border = '1px solid var(--color-search-bar-border)';
    searchBar.style.boxShadow = 'var(--shadow-search-box)';
    searchBar.style.borderRadius = '24px';
  }
  function onSearchTextChange() {
    // adjust the height
    textArea.style.height = 'auto';
    const h = textArea.scrollHeight + 6;
    textArea.style.height = `${h}px`;
    // display clear icon
    if (textArea.value !== '') {
      showElem(clearIcon);
    } else {
      hideElem(clearIcon);
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
  });
  clearIcon.addEventListener('click', () => {
    textArea.value = '';
    textArea.focus();
    hideElem(clearIcon);
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

window.addEventListener('beforeunload', (event) => {
  event.stopImmediatePropagation();
});
