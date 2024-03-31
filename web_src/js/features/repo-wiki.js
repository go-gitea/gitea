import {validateTextareaNonEmpty} from './comp/MarkdownEditor.js';
import {fomanticMobileScreen} from '../modules/fomantic.js';

async function initRepoWikiFormEditor() {
  const editArea = document.querySelector('.repository.wiki .markdown-editor textarea');
  if (!editArea) return;

  document.querySelector('.repository.wiki.new .ui.form').addEventListener('submit', (e) => {
    if (!validateTextareaNonEmpty(editArea)) {
      e.preventDefault();
      e.stopPropagation();
    }
  });
}

function collapseWikiTocForMobile(collapse) {
  if (collapse) {
    document.querySelector('.wiki-content-toc details')?.removeAttribute('open');
  }
}

export function initRepoWikiForm() {
  if (!document.querySelector('.page-content.repository.wiki')) return;

  fomanticMobileScreen.addEventListener('change', (e) => collapseWikiTocForMobile(e.matches));
  collapseWikiTocForMobile(fomanticMobileScreen.matches);

  initRepoWikiFormEditor();
}
