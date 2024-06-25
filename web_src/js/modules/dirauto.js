import {isDocumentFragmentOrElementNode} from '../utils/dom.js';

// for performance considerations, it only uses performant syntax
function attachDirAuto(el) {
  if (el.type !== 'hidden' &&
      el.type !== 'checkbox' &&
      el.type !== 'radio' &&
      el.type !== 'range' &&
      el.type !== 'color') {
    el.dir = 'auto';
  }
}

export function initDirAuto() {
  const observer = new MutationObserver((mutationList) => {
    const len = mutationList.length;
    for (let i = 0; i < len; i++) {
      const mutation = mutationList[i];
      const len = mutation.addedNodes.length;
      for (let i = 0; i < len; i++) {
        const addedNode = mutation.addedNodes[i];
        if (!isDocumentFragmentOrElementNode(addedNode)) continue;
        if (addedNode.nodeName === 'INPUT' || addedNode.nodeName === 'TEXTAREA') attachDirAuto(addedNode);
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
