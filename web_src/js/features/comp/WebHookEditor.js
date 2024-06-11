import {POST} from '../../modules/fetch.js';
import {hideElem, showElem, toggleElem} from '../../utils/dom.js';

export function initCompWebHookEditor() {
  if (!document.querySelectorAll('.new.webhook').length) {
    return;
  }

  for (const input of document.querySelectorAll('.events.checkbox input')) {
    input.addEventListener('change', function () {
      if (this.checked) {
        showElem('.events.fields');
      }
    });
  }

  for (const input of document.querySelectorAll('.non-events.checkbox input')) {
    input.addEventListener('change', function () {
      if (this.checked) {
        hideElem('.events.fields');
      }
    });
  }

  // some webhooks (like Gitea) allow to set the request method (GET/POST), and it would toggle the "Content Type" field
  const httpMethodInput = document.querySelector('#http_method');
  if (httpMethodInput) {
    const updateContentType = function () {
      const visible = httpMethodInput.value === 'POST';
      toggleElem(document.querySelector('#content_type').closest('.field'), visible);
    };
    updateContentType();
    httpMethodInput.addEventListener('change', updateContentType);
  }

  // Test delivery
  document.querySelector('#test-delivery')?.addEventListener('click', async function () {
    this.classList.add('is-loading', 'disabled');
    await POST(this.getAttribute('data-link'));
    setTimeout(() => {
      window.location.href = this.getAttribute('data-redirect');
    }, 5000);
  });
}
