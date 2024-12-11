import {getCurrentLocale} from '../utils.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

const {pageData} = window.config;

async function initInputCitationValue(citationCopyApa: HTMLButtonElement, citationCopyBibtex: HTMLButtonElement) {
  const [{Cite, plugins}] = await Promise.all([
    import(/* webpackChunkName: "citation-js-core" */'@citation-js/core'),
    import(/* webpackChunkName: "citation-js-formats" */'@citation-js/plugin-software-formats'),
    import(/* webpackChunkName: "citation-js-bibtex" */'@citation-js/plugin-bibtex'),
    import(/* webpackChunkName: "citation-js-csl" */'@citation-js/plugin-csl'),
  ]);
  const {citationFileContent} = pageData;
  const config = plugins.config.get('@bibtex');
  config.constants.fieldTypes.doi = ['field', 'literal'];
  config.constants.fieldTypes.version = ['field', 'literal'];
  const citationFormatter = new Cite(citationFileContent);
  const lang = getCurrentLocale() || 'en-US';
  const apaOutput = citationFormatter.format('bibliography', {template: 'apa', lang});
  const bibtexOutput = citationFormatter.format('bibtex', {lang});
  citationCopyBibtex.setAttribute('data-text', bibtexOutput);
  citationCopyApa.setAttribute('data-text', apaOutput);
}

export async function initCitationFileCopyContent() {
  const defaultCitationFormat = 'apa'; // apa or bibtex

  if (!pageData.citationFileContent) return;

  const citationCopyApa = document.querySelector<HTMLButtonElement>('#citation-copy-apa');
  const citationCopyBibtex = document.querySelector<HTMLButtonElement>('#citation-copy-bibtex');
  const inputContent = document.querySelector<HTMLInputElement>('#citation-copy-content');

  if ((!citationCopyApa && !citationCopyBibtex) || !inputContent) return;

  const updateUi = () => {
    const isBibtex = (localStorage.getItem('citation-copy-format') || defaultCitationFormat) === 'bibtex';
    const copyContent = (isBibtex ? citationCopyBibtex : citationCopyApa).getAttribute('data-text');
    inputContent.value = copyContent;
    citationCopyBibtex.classList.toggle('primary', isBibtex);
    citationCopyApa.classList.toggle('primary', !isBibtex);
  };

  document.querySelector('#cite-repo-button')?.addEventListener('click', async () => {
    try {
      await initInputCitationValue(citationCopyApa, citationCopyBibtex);
    } catch (e) {
      console.error(`initCitationFileCopyContent error: ${e}`, e);
      return;
    }
    updateUi();

    citationCopyApa.addEventListener('click', () => {
      localStorage.setItem('citation-copy-format', 'apa');
      updateUi();
    });

    citationCopyBibtex.addEventListener('click', () => {
      localStorage.setItem('citation-copy-format', 'bibtex');
      updateUi();
    });

    inputContent.addEventListener('click', () => {
      inputContent.select();
    });

    fomanticQuery('#cite-repo-modal').modal('show');
  });
}
