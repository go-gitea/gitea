import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import {createCodeEditor} from './codeeditor.js';
import {hideElem, queryElems, showElem} from '../utils/dom.js';
import {initMarkupContent} from '../markup/content.js';
import {attachRefIssueContextPopup} from './contextpopup.js';
import {POST} from '../modules/fetch.js';

function initEditPreviewTab($form) {
  const $tabMenu = $form.find('.repo-editor-menu');
  $tabMenu.find('.item').tab();
  const $previewTab = $tabMenu.find('a[data-tab="preview"]');
  if ($previewTab.length) {
    $previewTab.on('click', async function () {
      const $this = $(this);
      let context = `${$this.data('context')}/`;
      const mode = $this.data('markup-mode') || 'comment';
      const $treePathEl = $form.find('input#tree_path');
      if ($treePathEl.length > 0) {
        context += $treePathEl.val();
      }
      context = context.substring(0, context.lastIndexOf('/'));

      const formData = new FormData();
      formData.append('mode', mode);
      formData.append('context', context);
      formData.append('text', $form.find('.tab[data-tab="write"] textarea').val());
      formData.append('file_path', $treePathEl.val());
      try {
        const response = await POST($this.data('url'), {data: formData});
        const data = await response.text();
        const $previewPanel = $form.find('.tab[data-tab="preview"]');
        if ($previewPanel.length) {
          renderPreviewPanelContent($previewPanel, data);
        }
      } catch (error) {
        console.error('Error:', error);
      }
    });
  }
}

export function initRepoEditor() {
  const $editArea = $('.repository.editor textarea#edit_area');
  if (!$editArea.length) return;

  for (const el of queryElems('.js-quick-pull-choice-option')) {
    el.addEventListener('input', () => {
      if (el.value === 'commit-to-new-branch') {
        showElem('.quick-pull-branch-name');
        document.querySelector('.quick-pull-branch-name input').required = true;
      } else {
        hideElem('.quick-pull-branch-name');
        document.querySelector('.quick-pull-branch-name input').required = false;
      }
      document.querySelector('#commit-button').textContent = el.getAttribute('data-button-text');
    });
  }

  const filenameInput = document.querySelector('#file-name');
  function joinTreePath() {
    const parts = [];
    for (const el of document.querySelectorAll('.breadcrumb span.section')) {
      const link = el.querySelector('a');
      parts.push(link ? link.textContent : el.textContent);
    }
    if (filenameInput.value) {
      parts.push(filenameInput.value);
    }
    document.querySelector('#tree_path').value = parts.join('/');
  }
  filenameInput.addEventListener('input', function () {
    const parts = filenameInput.value.split('/');
    if (parts.length > 1) {
      for (let i = 0; i < parts.length; ++i) {
        const value = parts[i];
        if (i < parts.length - 1) {
          if (value.length) {
            $(`<span class="section"><a href="#">${htmlEscape(value)}</a></span>`).insertBefore($(filenameInput));
            $('<div class="breadcrumb-divider">/</div>').insertBefore($(filenameInput));
          }
        } else {
          filenameInput.value = value;
        }
        this.setSelectionRange(0, 0);
      }
    }
    joinTreePath();
  });
  filenameInput.addEventListener('keydown', function (e) {
    const sections = queryElems('.breadcrumb span.section');
    const dividers = queryElems('.breadcrumb .breadcrumb-divider');
    // Jump back to last directory once the filename is empty
    if (e.code === 'Backspace' && filenameInput.selectionStart === 0 && sections.length > 0) {
      e.preventDefault();
      const lastSection = sections[sections.length - 1];
      const lastDivider = dividers.length ? dividers[dividers.length - 1] : null;
      const value = lastSection.querySelector('a').textContent;
      filenameInput.value = value + filenameInput.value;
      this.setSelectionRange(value.length, value.length);
      lastDivider?.remove();
      lastSection.remove();
      joinTreePath();
    }
  });

  const $form = $('.repository.editor .edit.form');
  initEditPreviewTab($form);

  (async () => {
    const editor = await createCodeEditor($editArea[0], filenameInput);

    // Using events from https://github.com/codedance/jquery.AreYouSure#advanced-usage
    // to enable or disable the commit button
    const commitButton = document.querySelector('#commit-button');
    const $editForm = $('.ui.edit.form');
    const dirtyFileClass = 'dirty-file';

    // Disabling the button at the start
    if ($('input[name="page_has_posted"]').val() !== 'true') {
      commitButton.disabled = true;
    }

    // Registering a custom listener for the file path and the file content
    $editForm.areYouSure({
      silent: true,
      dirtyClass: dirtyFileClass,
      fieldSelector: ':input:not(.commit-form-wrapper :input)',
      change($form) {
        const dirty = $form[0]?.classList.contains(dirtyFileClass);
        commitButton.disabled = !dirty;
      },
    });

    // Update the editor from query params, if available,
    // only after the dirtyFileClass initialization
    const params = new URLSearchParams(window.location.search);
    const value = params.get('value');
    if (value) {
      editor.setValue(value);
    }

    commitButton?.addEventListener('click', (e) => {
      // A modal which asks if an empty file should be committed
      if (!$editArea.val()) {
        $('#edit-empty-content-modal').modal({
          onApprove() {
            $('.edit.form').trigger('submit');
          },
        }).modal('show');
        e.preventDefault();
      }
    });
  })();
}

export function renderPreviewPanelContent($previewPanel, data) {
  $previewPanel.html(data);
  initMarkupContent();

  const $refIssues = $previewPanel.find('p .ref-issue');
  attachRefIssueContextPopup($refIssues);
}
