import {request} from '../../modules/fetch.ts';

type TestResult = {ok: boolean; message: string; details?: string[]};

export function initAdminLDAPTestConnection(): void {
  for (const button of document.querySelectorAll<HTMLButtonElement>('[data-ldap-test-connection]')) {
    const form = button.closest('form')!;
    const result = form.querySelector<HTMLElement>('[data-ldap-test-result]')!;
    button.addEventListener('click', async () => {
      button.disabled = true;
      button.classList.add('loading');
      result.classList.add('tw-hidden');
      try {
        const response = await request(`${window.config.appSubUrl}/-/admin/auths/test-connection`, {method: 'POST', data: new FormData(form)});
        const data = await response.json() as TestResult;
        if (!response.ok) throw new Error(data.message || 'LDAP connection test failed');
        result.classList.remove('tw-hidden', 'positive', 'negative');
        result.classList.add(data.ok ? 'positive' : 'negative');
        result.replaceChildren();
        const message = document.createElement('strong');
        message.textContent = data.message;
        result.append(message);
        if (data.details?.length) {
          const details = document.createElement('ul');
          for (const detail of data.details) {
            const item = document.createElement('li');
            item.textContent = detail;
            details.append(item);
          }
          result.append(details);
        }
      } catch (error) {
        result.classList.remove('tw-hidden', 'positive');
        result.classList.add('negative');
        result.textContent = error instanceof Error ? error.message : String(error);
      } finally {
        button.disabled = false;
        button.classList.remove('loading');
      }
    });
  }
}
