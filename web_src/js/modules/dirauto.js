// for performance considerations, it only uses performant syntax

const excludeTypes = new Set(['hidden', 'checkbox', 'radio', 'range', 'color']);

function attachDirAuto(el) {
  if (!excludeTypes.has(el.type)) {
    el.dir = 'auto';
  }
}

export function initDirAuto() {
  const observer = new MutationObserver((mutationList) => {
    const len = mutationList.length;
    for (let i = 0; i < len; i++) {
      const mutation = mutationList[i];
      const len = mutation.addedNodes.length;
      for (let addedNodeIdx = 0; addedNodeIdx < len; addedNodeIdx++) {
        const addedNode = mutation.addedNodes[addedNodeIdx];
        if (addedNode.nodeType !== Node.ELEMENT_NODE && addedNode.nodeType !== Node.DOCUMENT_FRAGMENT_NODE) continue;
        attachDirAuto(addedNode);
        const children = addedNode.querySelectorAll('input, textarea');
        const len = children.length;
        for (let childIdx = 0; childIdx < len; childIdx++) {
          attachDirAuto(children[childIdx]);
        }
      }
    }
  });

  const docNodes = document.querySelectorAll('input, textarea');
  const len = docNodes.length;
  for (let i = 0; i < len; i++) {
    attachDirAuto(docNodes[i]);
  }

  observer.observe(document, {subtree: true, childList: true});
}
