import {validateTextareaNonEmpty, initComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {fomanticMobileScreen} from '../modules/fomantic.ts';

async function initRepoWikiFormEditor() {
  const editArea = document.querySelector<HTMLTextAreaElement>('.repository.wiki .combo-markdown-editor textarea');
  if (!editArea) return;

  const form = document.querySelector('.repository.wiki.new .ui.form')!;
  const editorContainer = form.querySelector<HTMLElement>('.combo-markdown-editor')!;

  await initComboMarkdownEditor(editorContainer);

  form.addEventListener('submit', (e) => {
    if (!validateTextareaNonEmpty(editArea)) {
      e.preventDefault();
      e.stopPropagation();
    }
  });
}

function collapseWikiTocForMobile(collapse: boolean) {
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
