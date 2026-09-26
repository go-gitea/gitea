import {localUserSettings} from '../modules/user-settings.ts';

export function initCitationFileCopyContent() {
  const citationCopyBibtex = document.querySelector<HTMLButtonElement>('#citation-copy-bibtex');
  if (!citationCopyBibtex) return;

  const citationCopyApa = document.querySelector<HTMLButtonElement>('#citation-copy-apa');
  const inputContent = document.querySelector<HTMLInputElement>('#citation-copy-content')!;
  const clipboardBtn = document.querySelector('#citation-clipboard-btn')!;

  const updateUi = () => {
    const isBibtex = !citationCopyApa || localUserSettings.getString('citation-copy-format', 'apa') === 'bibtex';
    const text = (isBibtex ? citationCopyBibtex : citationCopyApa).getAttribute('data-text')!;
    inputContent.value = text;
    inputContent.setSelectionRange(0, 0);
    clipboardBtn.setAttribute('data-clipboard-text', text); // the input strips newlines
    citationCopyBibtex.classList.toggle('primary', isBibtex);
    citationCopyApa?.classList.toggle('primary', !isBibtex);
  };

  for (const [format, button] of Object.entries({apa: citationCopyApa, bibtex: citationCopyBibtex})) {
    button?.addEventListener('click', () => {
      localUserSettings.setString('citation-copy-format', format);
      updateUi();
    });
  }

  updateUi();
}
