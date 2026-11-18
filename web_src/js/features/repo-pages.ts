import {registerGlobalInitFunc} from '../modules/observer.ts';
import {toggleElem} from '../utils/dom.ts';
import {DropzoneCustomEventUploadDone, initDropzone} from './dropzone.ts';

export function initRepoPagesSettings(elRoot: HTMLElement) {
  const sourceSelect = elRoot.querySelector<HTMLSelectElement>('#pages-source-type')!;
  const updatePanels = () => {
    for (const panel of elRoot.querySelectorAll<HTMLElement>('[data-pages-source-panel]')) {
      const active = panel.getAttribute('data-pages-source-panel') === sourceSelect.value;
      toggleElem(panel, active);
      for (const control of panel.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement | HTMLButtonElement>('input, select, textarea, button')) {
        control.disabled = !active;
      }
    }
  };
  sourceSelect.addEventListener('change', updatePanels);
  updatePanels();
}

export async function initRepoPagesVersions(elRoot: HTMLElement) {
  const dropzone = elRoot.querySelector<HTMLElement>('.dropzone');
  if (!dropzone) return;

  const dropzoneInstance = await initDropzone(dropzone);
  const uploadForm = dropzone.closest<HTMLFormElement>('[data-pages-upload]')!;
  dropzoneInstance.on('addedfile', () => dropzone.classList.add('loading', 'disabled'));
  dropzoneInstance.on('error', () => dropzone.classList.remove('loading', 'disabled'));
  dropzoneInstance.on(DropzoneCustomEventUploadDone, () => uploadForm.requestSubmit());
}

export function initRepoPages() {
  registerGlobalInitFunc('initRepoPagesSettings', initRepoPagesSettings);
  registerGlobalInitFunc('initRepoPagesVersions', initRepoPagesVersions);
}
