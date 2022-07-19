import $ from 'jquery';

const {pageData} = window.config;

const initInputCitationValue = async () => {
  const [{Cite, plugins}] = await Promise.all([
    import(/* webpackChunkName: "citation-js-core" */'@citation-js/core'),
    import(/* webpackChunkName: "citation-js-formats" */'@citation-js/plugin-software-formats'),
    import(/* webpackChunkName: "citation-js-bibtex" */'@citation-js/plugin-bibtex'),
    import(/* webpackChunkName: "citation-js-bibtex" */'@citation-js/plugin-csl'),
  ]);
  const {citiationFileContent} = pageData;
  const $citationCopyApa = $('#citation-copy-apa');
  const $citationCopyBibtex = $('#citation-copy-bibtex');
  const config = plugins.config.get('@bibtex');
  config.constants.fieldTypes.doi = ['field', 'literal'];
  config.constants.fieldTypes.version = ['field', 'literal'];
  const citationFormatter = new Cite(citiationFileContent);
  const apaOutput = citationFormatter.format('bibliography', {
    template: 'apa',
    lang: document.documentElement.lang || 'en-US'
  });
  const bibtexOutput = citationFormatter.format('bibtex', {
    lang: document.documentElement.lang || 'en-US'
  });
  $citationCopyBibtex.attr('data-text', bibtexOutput);
  $citationCopyApa.attr('data-text', apaOutput);
};

export function initCitationFileCopyContent() {
  const defaultCitationFormat = 'apa'; // apa or bibtex

  const $citationCopyApa = $('#citation-copy-apa');
  const $citationCopyBibtex = $('#citation-copy-bibtex');
  const $inputContent = $('#citation-copy-content');

  if ((!$citationCopyApa.length && !$citationCopyBibtex.length) || !$inputContent.length) {
    return;
  }
  const updateUi = () => {
    const isBibtex = (localStorage.getItem('citation-copy-format') || defaultCitationFormat) === 'bibtex';
    const copyContent = (isBibtex ? $citationCopyBibtex : $citationCopyApa).attr('data-text');

    $inputContent.val(copyContent);
    $citationCopyBibtex.toggleClass('primary', isBibtex);
    $citationCopyApa.toggleClass('primary', !isBibtex);
  };
  initInputCitationValue().then(updateUi);

  setTimeout(() => {
    // restore animation after first init
    $citationCopyApa.removeClass('no-transition');
    $citationCopyBibtex.removeClass('no-transition');
  }, 100);

  $citationCopyApa.on('click', () => {
    localStorage.setItem('citation-copy-format', 'apa');
    updateUi();
  });
  $citationCopyBibtex.on('click', () => {
    localStorage.setItem('citation-copy-format', 'bibtex');
    updateUi();
  });

  $inputContent.on('click', () => {
    $inputContent.select();
  });
}
