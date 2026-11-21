import {registerGlobalInitFunc} from '../../modules/observer.ts';
import {queryElems, toggleElem} from '../../utils/dom.ts';

function initCompWebHookEditorForm(el: HTMLElement) {
  const elCustomEvents = el.querySelector('.js-webhook-custom-events')!;

  // Trigger On: use custom events or not
  queryElems<HTMLInputElement>(el, 'input[name=events]', (input) => input.addEventListener('change', () => {
    if (!input.checked) return;
    toggleElem(elCustomEvents, input.value === 'choose_events');
  }));

  // Select All / Deselect All custom events
  queryElems(el, 'button[data-events-select-all]', (btn) => btn.addEventListener('click', () => {
    const checked = btn.getAttribute('data-events-select-all') === 'true';
    queryElems<HTMLInputElement>(elCustomEvents, 'input[type=checkbox]', (input) => {
      input.checked = checked;
      input.dispatchEvent(new Event('change', {bubbles: true})); // for the areYouSure dirty tracking
    });
  }));

  // some webhooks (like Gitea) allow to set the request method (GET/POST), and it would toggle the "Content Type" field
  const httpMethodInput = el.querySelector<HTMLInputElement>('#http_method');
  if (httpMethodInput) {
    const updateContentType = () => {
      const visible = httpMethodInput.value === 'POST';
      toggleElem(el.querySelector('#content_type')!.closest('.field')!, visible);
    };
    updateContentType();
    httpMethodInput.addEventListener('change', updateContentType);
  }
}

export function initCompWebHookEditor() {
  registerGlobalInitFunc('initCompWebHookEditorForm', initCompWebHookEditorForm);
}
