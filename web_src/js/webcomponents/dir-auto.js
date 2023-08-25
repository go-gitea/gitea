const excludeInputTypes = new Set(['hidden', 'checkbox', 'radio', 'range', 'color']);
const includeNodeTypes = new Set([Node.ELEMENT_NODE, Node.DOCUMENT_FRAGMENT_NODE]);

let timeSpent = 0;

const observer = new MutationObserver((mutationList) => {
  const start = performance.now();
  for (const mutation of mutationList) {
    for (const el of mutation.addedNodes) {
      if (!includeNodeTypes.has(el.nodeType)) continue;
      if (!el.querySelector('input, textarea')) continue;
      for (const child of el.querySelectorAll('input, textarea')) {
        if (excludeInputTypes.has(child.type)) continue;
        child.dir = 'auto';
      }
    }
  }
  timeSpent += performance.now() - start;
});

setTimeout(() => console.log(timeSpent), 2000);

observer.observe(document, {subtree: true, childList: true});
