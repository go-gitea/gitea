import {localUserSettings} from '../modules/user-settings.ts';

export function initCitationFileCopyContent() {
  const citationCopyBibtex = document.querySelector<HTMLButtonElement>('#citation-copy-bibtex');
  if (!citationCopyBibtex) return;

  const citationCopyApa = document.querySelector<HTMLButtonElement>('#citation-copy-apa');
  const inputContent = document.querySelector<HTMLInputElement>('#citation-copy-content')!;

  const updateUi = () => {
    const isBibtex = !citationCopyApa || localUserSettings.getString('citation-copy-format', 'apa') === 'bibtex';
    inputContent.value = (isBibtex ? citationCopyBibtex : citationCopyApa).getAttribute('data-text')!;
    citationCopyBibtex.classList.toggle('primary', isBibtex);
    citationCopyApa?.classList.toggle('primary', !isBibtex);
  };

  for (const [format, button] of Object.entries({apa: citationCopyApa, bibtex: citationCopyBibtex})) {
    button?.addEventListener('click', () => {
      localUserSettings.setString('citation-copy-format', format);
      updateUi();
    });
  }

  inputContent.addEventListener('click', () => {
    inputContent.select();
  });

  updateUi();
  inputContent.select(); // kept until the modal autofocuses the input
}
