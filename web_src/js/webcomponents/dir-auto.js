const excludeInputTypes = new Set(['hidden', 'checkbox', 'radio', 'range', 'color']);

const observer = new MutationObserver((mutationList) => {
  for (const mutation of mutationList) {
    for (const el of mutation.addedNodes) {
      if (el.nodeType === Node.ELEMENT_NODE || el.nodeType === Node.DOCUMENT_FRAGMENT_NODE) {
        for (const child of el.querySelectorAll('input, textarea')) {
          if (excludeInputTypes.has(child.type)) continue;
          child.setAttribute('dir', 'auto');
        }
      }
    }
  }
});
observer.observe(document, {subtree: true, childList: true});
