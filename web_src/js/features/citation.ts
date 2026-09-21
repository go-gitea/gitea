import {getCurrentLocale} from '../utils.ts';
import {errorMessage} from '../modules/errors.ts';
import {showFomanticModal} from '../modules/fomantic/modal.ts';
import {localUserSettings} from '../modules/user-settings.ts';

const {pageData} = window.config;

export async function formatCitations(citationFileContent: string, lang: string) {
  const [{Cite}] = await Promise.all([
    import('@citation-js/core'),
    import('@citation-js/plugin-software-formats'),
    import('@citation-js/plugin-bibtex'),
    import('@citation-js/plugin-csl'),
  ]);
  const citationFormatter = new Cite(citationFileContent);
  for (const item of citationFormatter.data) { // citeproc-js crashes on numeric variables like CFF's "volume: 5"
    for (const [key, value] of Object.entries(item)) {
      if (typeof value === 'number') Object.assign(item, {[key]: String(value)});
    }
  }
  return {
    apa: citationFormatter.format('bibliography', {style: 'apa', lang}),
    bibtex: citationFormatter.format('bibtex'),
  };
}

export async function initCitationFileCopyContent() {
  const defaultCitationFormat = 'apa'; // apa or bibtex

  if (!pageData.citationFileContent) return;

  const citeRepoButton = document.querySelector('#cite-repo-button');
  if (!citeRepoButton) return; // sidebar is hidden

  const citationCopyApa = document.querySelector<HTMLButtonElement>('#citation-copy-apa')!;
  const citationCopyBibtex = document.querySelector<HTMLButtonElement>('#citation-copy-bibtex')!;
  const inputContent = document.querySelector<HTMLInputElement>('#citation-copy-content')!;

  const updateUi = () => {
    const isBibtex = localUserSettings.getString('citation-copy-format', defaultCitationFormat) === 'bibtex';
    const copyContent = (isBibtex ? citationCopyBibtex : citationCopyApa).getAttribute('data-text')!;
    inputContent.value = copyContent;
    citationCopyBibtex.classList.toggle('primary', isBibtex);
    citationCopyApa.classList.toggle('primary', !isBibtex);
  };

  citationCopyApa.addEventListener('click', () => {
    localUserSettings.setString('citation-copy-format', 'apa');
    updateUi();
  });

  citationCopyBibtex.addEventListener('click', () => {
    localUserSettings.setString('citation-copy-format', 'bibtex');
    updateUi();
  });

  inputContent.addEventListener('click', () => {
    inputContent.select();
  });

  citeRepoButton.addEventListener('click', async () => {
    try {
      const {apa, bibtex} = await formatCitations(pageData.citationFileContent!, getCurrentLocale() || 'en-US');
      citationCopyApa.setAttribute('data-text', apa);
      citationCopyBibtex.setAttribute('data-text', bibtex);
    } catch (e) {
      console.error(`initCitationFileCopyContent error: ${errorMessage(e)}`, e);
      return;
    }
    updateUi();
    showFomanticModal(document.querySelector('#cite-repo-modal'));
  });
}
