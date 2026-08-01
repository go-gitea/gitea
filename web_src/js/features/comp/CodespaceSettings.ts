import {attachSearchBox} from '../../modules/search.ts';
import {hideElem, showElem} from '../../utils/dom.ts';
import {svg} from '../../svg.ts';

type RepositorySearchResponse = {data: Array<{id: number, full_name: string}>};
type SelectedRepository = {id: string, name: string};

export function initCodespaceSecretRepositoryPicker(root: HTMLElement) {
  const checkbox = root.querySelector<HTMLInputElement>('.codespace-secret-all-repositories')!;
  const selectedScope = root.querySelector<HTMLElement>('.codespace-secret-selected-scope')!;
  const search = root.querySelector<HTMLElement>('.codespace-secret-repository-search')!;
  const searchInput = search.querySelector<HTMLInputElement>('input')!;
  const list = root.querySelector<HTMLElement>('.codespace-secret-selected-repositories')!;
  const selected = new Map<string, SelectedRepository>();

  const render = () => {
    list.replaceChildren(...Array.from(selected.values(), (repository) => {
      const item = document.createElement('div');
      item.className = 'item tw-flex tw-items-center tw-gap-2';
      const name = document.createElement('span');
      name.className = 'tw-flex-1 tw-break-anywhere';
      name.textContent = repository.name;
      const input = document.createElement('input');
      input.type = 'hidden';
      input.name = 'repository_ids';
      input.value = repository.id;
      const remove = document.createElement('button');
      remove.type = 'button';
      remove.className = 'btn interact-bg tw-p-2';
      remove.setAttribute('aria-label', root.getAttribute('data-remove-label')!);
      remove.setAttribute('data-tooltip-content', root.getAttribute('data-remove-label')!);
      remove.innerHTML = svg('octicon-x');
      remove.addEventListener('click', () => {
        selected.delete(repository.id);
        render();
      });
      item.append(input, name, remove);
      return item;
    }));
  };

  const reset = (repositories: SelectedRepository[], allRepositories: boolean) => {
    selected.clear();
    for (const repository of repositories) selected.set(repository.id, repository);
    checkbox.checked = allRepositories;
    searchInput.value = '';
    if (allRepositories) hideElem(selectedScope); else showElem(selectedScope);
    render();
  };

  checkbox.addEventListener('change', () => {
    if (checkbox.checked) hideElem(selectedScope); else showElem(selectedScope);
  });
  attachSearchBox(search, `${root.getAttribute('data-search-url')}?q={query}`, (response: RepositorySearchResponse) => response.data.map((repository) => ({
    title: repository.full_name,
    description: repository.full_name,
    value: String(repository.id),
  })), {
    onSelect(result) {
      selected.set(result.value!, {id: result.value!, name: result.title});
      searchInput.value = '';
      render();
    },
  });

  if (root.closest('#codespace-secret-create-modal')) {
    document.querySelector('.codespace-secret-create-button')!.addEventListener('click', () => {
      root.closest('form')!.reset();
      reset([], false);
    });
    for (const button of document.querySelectorAll<HTMLElement>('[data-modal="#codespace-secret-value-modal"]')) {
      button.addEventListener('click', () => {
        document.querySelector<HTMLTextAreaElement>('#codespace-secret-new-value')!.value = '';
      });
    }
  } else {
    for (const button of document.querySelectorAll<HTMLElement>('.codespace-secret-access-button')) {
      button.addEventListener('click', () => {
        const item = button.closest<HTMLElement>('.codespace-secret-item')!;
        const repositories = Array.from(item.querySelectorAll<HTMLElement>('[data-repository-id]'), (element) => ({
          id: element.getAttribute('data-repository-id')!,
          name: element.getAttribute('data-repository-name')!,
        }));
        reset(repositories, button.getAttribute('data-modal-secret-access-all.checked') === 'true');
      });
    }
  }
  reset([], false);
}
