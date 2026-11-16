import {showModal} from '../../modules/modal.ts';
import {hideElem, showElem, toggleElem} from '../../utils/dom.ts';
import {htmlEscape} from '../../utils/html.ts';
import {fomanticQuery} from '../../modules/fomantic/base.ts';
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
  const modal = root.closest('dialog')!;
  const checkbox = root.querySelector<HTMLInputElement>('.codespace-secret-all-repositories')!;
  const selectedScope = root.querySelector<HTMLElement>('.codespace-secret-selected-scope')!;
  const select = root.querySelector<HTMLSelectElement>('select[name="repository_ids"]')!;
  const $dropdown = fomanticQuery(select).dropdown({
    forceSelection: false,
    allowAdditions: false,
    message: {
      noResults: root.getAttribute('data-no-results')!,
      serverError: root.getAttribute('data-search-error')!,
    },
    apiSettings: {
      url: `${root.getAttribute('data-search-url')}?q={query}`,
      cache: false,
      beforeSend: (settings: unknown) => modal.open && !checkbox.checked && $dropdown[0].contains(document.activeElement) ? settings : false,
      onResponse: (response: RepositorySearchResponse) => ({results: response.data.map((repository) => ({name: htmlEscape(repository.full_name), value: String(repository.id)}))}),
      onFailure() {
        $dropdown.dropdown('setup menu', {values: []});
        $dropdown.dropdown('add message', root.getAttribute('data-search-error')!);
        $dropdown.dropdown('show');
      },
    },
    onShow: () => modal.open && !checkbox.checked && $dropdown[0].contains(document.activeElement),
    onHide() {
      if ($dropdown.data('module-api')) $dropdown.api('abort');
    },
  });
  $dropdown.find('input.search').attr('aria-label', select.getAttribute('aria-label')!);
  root.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !event.isComposing && $dropdown.dropdown('is visible')) event.preventDefault();
  }, {capture: true});

  const clearSearch = () => {
    if ($dropdown.data('module-api')) $dropdown.api('abort');
    $dropdown.dropdown('hide');
    $dropdown.dropdown('remove search term');
    $dropdown.dropdown('remove message');
    $dropdown.dropdown('setup menu', {values: []});
  };

  const updateScope = () => {
    select.disabled = checkbox.checked;
    $dropdown.toggleClass('disabled', checkbox.checked);
    toggleElem(selectedScope, !checkbox.checked);
    if (checkbox.checked) clearSearch();
  };

  const reset = (repositories: SelectedRepository[], allRepositories: boolean) => {
    $dropdown.children('.ui.label').remove(); // Form reset may already have cleared the select's values.
    $dropdown.dropdown('change values', repositories.map((repository) => ({name: htmlEscape(repository.name), value: repository.id})));
    $dropdown.dropdown('set selected', repositories.map((repository) => repository.id));
    clearSearch();
    checkbox.checked = allRepositories;
    updateScope();
  };

  checkbox.addEventListener('change', updateScope);
  modal.addEventListener('close', clearSearch);

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
        reset(repositories, button.getAttribute('data-modal-codespace-secret-access-all') === 'true');
      });
    }
  }
  reset([], false);
}
