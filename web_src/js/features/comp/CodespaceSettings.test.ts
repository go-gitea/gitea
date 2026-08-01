import {initCodespaceSecretRepositoryPicker} from './CodespaceSettings.ts';

test('secret repository access loads the complete selected scope and switches to all repositories', () => {
  document.body.innerHTML = `
    <div class="codespace-secret-item">
      <span data-repository-id="1" data-repository-name="owner/first"></span>
      <span data-repository-id="2" data-repository-name="owner/second"></span>
      <button class="codespace-secret-access-button" data-modal-secret-access-all.checked="false"></button>
    </div>
    <div id="codespace-secret-access-modal">
      <form>
        <div class="codespace-secret-repository-picker" data-search-url="/repositories" data-remove-label="Remove">
          <input class="codespace-secret-all-repositories" type="checkbox">
          <div class="codespace-secret-selected-scope">
            <div class="codespace-secret-repository-search"><input class="prompt"></div>
            <div class="codespace-secret-selected-repositories"></div>
          </div>
        </div>
      </form>
    </div>`;

  const picker = document.querySelector<HTMLElement>('.codespace-secret-repository-picker')!;
  initCodespaceSecretRepositoryPicker(picker);
  document.querySelector<HTMLButtonElement>('.codespace-secret-access-button')!.click();

  const selected = Array.from(picker.querySelectorAll<HTMLInputElement>('input[name="repository_ids"]'), (input) => input.value);
  expect(selected).toEqual(['1', '2']);

  const allRepositories = picker.querySelector<HTMLInputElement>('.codespace-secret-all-repositories')!;
  allRepositories.checked = true;
  allRepositories.dispatchEvent(new Event('change'));
  expect(picker.querySelector('.codespace-secret-selected-scope')!.classList.contains('tw-hidden')).toBe(true);
});
