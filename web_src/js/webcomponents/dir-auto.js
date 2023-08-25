const observer = new MutationObserver((mutationList) => {
  for (const mutation of mutationList) {
    for (const el of mutation.addedNodes) {
      if (el.nodeType === Node.ELEMENT_NODE || el.nodeType === Node.DOCUMENT_FRAGMENT_NODE) {
        for (const child of el.querySelectorAll('input:not([dir]), textarea:not([dir])')) {
          child.setAttribute('dir', 'auto');
        }
      }
    }
  }
});
observer.observe(document, {subtree: true, childList: true});
