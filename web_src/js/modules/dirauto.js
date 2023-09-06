// for performance considerations, it only uses performant syntax

const excludeTypes = new Set(['hidden', 'checkbox', 'radio', 'range', 'color']);

function attachDirAuto(el) {
  if (!excludeTypes.has(el.type)) {
    el.dir = 'auto';
  }
}

export function initDirAuto() {
  let timeSpent = 0;

  const observer = new MutationObserver((mutationList) => {
    const start = performance.now();
    for (let i = 0; i < mutationList.length; i++) {
      const mutation = mutationList[i];
      for (let addedNodeIdx = 0; addedNodeIdx < mutation.addedNodes.length; addedNodeIdx++) {
        const addedNode = mutation.addedNodes[addedNodeIdx];
        if (addedNode.nodeType !== Node.ELEMENT_NODE && addedNode.nodeType !== Node.DOCUMENT_FRAGMENT_NODE) continue;
        attachDirAuto(addedNode);
        const children = addedNode.querySelectorAll('input, textarea');
        for (let childIdx = 0; childIdx < children.length; childIdx++) {
          attachDirAuto(children[childIdx]);
        }
      }
    }
    timeSpent += performance.now() - start;
  });

  const start = performance.now();
  const docNodes = document.querySelectorAll('input, textarea');
  for (let i = 0; i < docNodes.length; i++) {
    attachDirAuto(docNodes[i]);
  }
  timeSpent += performance.now() - start;

  setTimeout(() => console.log(timeSpent), 2000);

  observer.observe(document, {subtree: true, childList: true});
}
