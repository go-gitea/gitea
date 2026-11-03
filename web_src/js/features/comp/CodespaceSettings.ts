import {attachSearchBox} from '../../modules/search.ts';
import {showModal} from '../../modules/modal.ts';
import {hideElem, showElem} from '../../utils/dom.ts';
import {svg} from '../../svg.ts';
import {createCodeEditor, type CodemirrorEditor} from '../../modules/codeeditor/main.ts';

export function initCodespaceTemplateEditor(modal: HTMLDialogElement) {
  const form = modal.querySelector<HTMLFormElement>('form')!;
  const textarea = form.querySelector<HTMLTextAreaElement>('[name="content"]')!;
  const name = form.querySelector<HTMLInputElement>('[name="name"]')!;
  const submit = form.querySelector<HTMLButtonElement>('.ok')!;
  const cancel = form.querySelector<HTMLButtonElement>('.cancel')!;
  const triggers = document.querySelectorAll<HTMLButtonElement>('.codespace-template-edit');
  let editor: CodemirrorEditor | undefined;
  for (const button of triggers) {
    button.addEventListener('click', () => {
      form.action = button.getAttribute('data-modal-form.url')!;
      name.value = button.getAttribute('data-modal-codespace-devcontainer-template-name')!;
      textarea.defaultValue = textarea.value = button.getAttribute('data-modal-codespace-devcontainer-template-content')!;
      modal.querySelector('.header')!.textContent = button.getAttribute('data-modal-template-modal-title')!;
      submit.querySelector('.template-submit-label')!.textContent = button.getAttribute('data-submit-label')!;
      for (const field of form.querySelectorAll('.field.error')) field.classList.remove('error');
      showModal(modal, {
        async onShow() {
          for (const trigger of triggers) trigger.disabled = true;
          submit.disabled = cancel.disabled = true;
          textarea.readOnly = true;
          try {
            const created = await createCodeEditor(textarea);
            if (!modal.open) {
              created.view.destroy();
              textarea.parentElement!.querySelector('.code-editor-container')?.remove();
              return;
            }
            editor = created;
            hideElem(textarea);
            editor.view.requestMeasure();
            name.focus();
          } catch {
            textarea.parentElement!.querySelector('.code-editor-container')?.remove();
            showElem(textarea);
          } finally {
            for (const trigger of triggers) trigger.disabled = false;
            textarea.readOnly = false;
            submit.disabled = cancel.disabled = false;
          }
        },
        onHidden() {
          editor?.view.destroy();
          editor = undefined;
          textarea.parentElement!.querySelector('.code-editor-container')?.remove();
          showElem(textarea);
        },
      });
    });
  }
}

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
