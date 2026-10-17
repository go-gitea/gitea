import {DiffViewer} from '../modules/diffviewer/viewer.ts';
import {initDiffFileTree} from './repo-diff-filetree.ts';
import {initDiffCommitSelect} from './repo-diff-commitselect.ts';
import {initImageDiff} from './imagediff.ts';
import {createTippy} from '../modules/tippy.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {hideElem, queryElemSiblings, queryElems, showElem} from '../utils/dom.ts';

// switches between "rendered" and "source", for image and CSV files
function initDiffFileViewToggle(el: HTMLElement) {
  queryElems(el, '.file-view-toggle', (btn) => btn.addEventListener('click', () => {
    queryElemSiblings(btn, '.file-view-toggle', (el) => el.classList.remove('active'));
    btn.classList.add('active');

    const target = document.querySelector(btn.getAttribute('data-toggle-selector')!);
    if (!target) throw new Error('Target element not found');

    hideElem(queryElemSiblings(target));
    showElem(target);
  }));
}

function initDiffHeaderPopupMenu(btn: HTMLElement) {
  const popup = btn.nextElementSibling;
  if (!popup?.matches('.tippy-target')) throw new Error('Popup element not found');
  createTippy(btn, {
    content: popup,
    theme: 'menu',
    placement: 'bottom-end',
    trigger: 'click',
    interactive: true,
    hideOnClick: true,
  });
}

export async function initRepoDiffViewer(): Promise<void> {
  const el = document.querySelector<HTMLElement>('#diff-viewer');
  if (!el) return;
  registerGlobalInitFunc('initDiffHeaderPopupMenu', initDiffHeaderPopupMenu);
  registerGlobalInitFunc('initDiffFileImageDiff', initImageDiff);
  registerGlobalInitFunc('initDiffFileViewToggle', initDiffFileViewToggle);
  initDiffFileTree();
  initDiffCommitSelect();
  const viewer = new DiffViewer(el, window.config.pageData.diffViewer!);
  document.querySelector('#expand-files-btn')?.addEventListener('click', () => viewer.setAllFolded(false));
  document.querySelector('#collapse-files-btn')?.addEventListener('click', () => viewer.setAllFolded(true));
  await viewer.init();
}
