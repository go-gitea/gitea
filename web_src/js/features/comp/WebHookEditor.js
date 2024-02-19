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

  const updateContentType = function () {
    const visible = document.getElementById('http_method').value === 'POST';
    toggleElem(document.getElementById('content_type').parentNode.parentNode, visible);
  };
  updateContentType();

  document.getElementById('http_method').addEventListener('change', updateContentType);

  // Test delivery
  document.getElementById('test-delivery')?.addEventListener('click', async function () {
    this.classList.add('loading', 'disabled');
    await POST(this.getAttribute('data-link'));
    setTimeout(() => {
      window.location.href = this.getAttribute('data-redirect');
    }, 5000);
  });
}
