import {attachSearchBox} from '../../modules/search.ts';
import {showFomanticModal} from '../../modules/fomantic/modal.ts';
import {hideElem, showElem} from '../../utils/dom.ts';
import {svg} from '../../svg.ts';
import {performFetchActionRequest} from '../../modules/fetch-action.ts';
import {showErrorToast} from '../../modules/toast.ts';
import {createCodeEditor, type CodemirrorEditor} from '../../modules/codeeditor/main.ts';

export function initCodespaceTemplateEditor(modal: HTMLElement) {
  const form = modal.querySelector<HTMLFormElement>('form')!;
  const textarea = form.querySelector<HTMLTextAreaElement>('[name="content"]')!;
  const name = form.querySelector<HTMLInputElement>('[name="name"]')!;
  const submit = form.querySelector<HTMLButtonElement>('.ok')!;
  const cancel = form.querySelector<HTMLButtonElement>('.cancel')!;
  let editor: CodemirrorEditor | undefined;
  for (const button of document.querySelectorAll<HTMLButtonElement>('.codespace-template-edit')) {
    button.addEventListener('click', () => {
      form.action = button.getAttribute('data-modal-form.url')!;
      name.value = button.getAttribute('data-modal-codespace-devcontainer-template-name')!;
      textarea.defaultValue = textarea.value = button.getAttribute('data-modal-codespace-devcontainer-template-content')!;
      modal.querySelector('.header')!.textContent = button.getAttribute('data-modal-template-modal-title')!;
      submit.querySelector('.template-submit-label')!.textContent = button.getAttribute('data-submit-label')!;
      for (const field of form.querySelectorAll('.field.error')) field.classList.remove('error');
      showFomanticModal(modal, {
        closable: false,
        async onShow() {
          submit.disabled = cancel.disabled = true;
          textarea.readOnly = true;
          try {
            editor = await createCodeEditor(textarea);
            hideElem(textarea);
            editor.view.requestMeasure();
            name.focus();
          } catch {
            textarea.parentElement!.querySelector('.code-editor-container')?.remove();
            showElem(textarea);
          } finally {
            textarea.readOnly = false;
            submit.disabled = cancel.disabled = false;
          }
        },
        onHide() {
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

export function initCodespaceManagerSecretModal(modal: HTMLElement) {
  const trigger = document.querySelector<HTMLButtonElement>('#manager-secret-button')!;
  const status = document.querySelector<HTMLElement>('#manager-secret-status')!;
  const form = modal.querySelector<HTMLFormElement>('form')!;
  const submit = form.querySelector<HTMLButtonElement>('[data-secret-submit]')!;
  const close = form.querySelector<HTMLButtonElement>('.cancel')!;
  const value = modal.querySelector<HTMLInputElement>('#manager-secret-value')!;
  const result = modal.querySelector<HTMLElement>('[data-secret-result]')!;
  const confirmation = modal.querySelector<HTMLElement>('[data-secret-confirm]')!;
  let pending = false;
  let hasSecret = trigger.getAttribute('data-has-secret') === 'true';

  const generate = async () => {
    if (pending || !result.hidden) return;
    pending = true;
    submit.disabled = close.disabled = trigger.disabled = true;
    const data = new FormData(form);
    if (hasSecret) data.set('confirm', 'reset-secret');
    // A lost response may still have replaced the verifier; the next attempt needs confirmation.
    hasSecret = true;
    trigger.textContent = trigger.getAttribute('data-reset-label')!;
    try {
      const response = await performFetchActionRequest(form, {method: 'POST', url: form.action, data});
      if (!response) return;
      const body = await response.json();
      if (typeof body.secret !== 'string' || !body.secret) throw new Error('Missing secret');
      value.value = body.secret;
      result.hidden = false;
      confirmation.hidden = true;
      status.textContent = status.getAttribute('data-generated-label')!;
    } catch {
      showErrorToast(modal.getAttribute('data-response-error')!);
    } finally {
      pending = false;
      submit.disabled = close.disabled = trigger.disabled = false;
      if (result.hidden) showElem(submit); else hideElem(submit);
    }
  };
  form.addEventListener('submit', (event) => {
    event.preventDefault();
    generate();
  });
  trigger.addEventListener('click', () => {
    showFomanticModal(modal, {
      closable: false,
      onShow() {
        value.value = '';
        result.hidden = true;
        confirmation.hidden = !hasSecret;
        if (hasSecret) showElem(submit); else hideElem(submit);
        if (!hasSecret) generate();
      },
      onHide() {
        value.value = '';
        result.hidden = true;
      },
    });
  });
}
