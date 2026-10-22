import {localUserSettings} from '../modules/user-settings.ts';

export function initRepoCitationPanel(elPanel: HTMLElement) {
  const citationCopyApa = elPanel.querySelector<HTMLButtonElement>('.citation-apa');
  const citationCopyBibtex = elPanel.querySelector<HTMLButtonElement>('.citation-bibtex')!;
  const inputContent = elPanel.querySelector('input')!;
  const clipboardBtn = elPanel.querySelector('.citation-copy')!;

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
