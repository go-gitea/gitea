import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import {createCodeEditor} from './codeeditor.js';
import {hideElem, showElem} from '../utils/dom.js';
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

function initEditorForm() {
  const $form = $('.repository .edit.form');
  if (!$form) return;
  initEditPreviewTab($form);
}

function getCursorPosition($e) {
  const el = $e.get(0);
  let pos = 0;
  if ('selectionStart' in el) {
    pos = el.selectionStart;
  } else if ('selection' in document) {
    el.focus();
    const Sel = document.selection.createRange();
    const SelLength = document.selection.createRange().text.length;
    Sel.moveStart('character', -el.value.length);
    pos = Sel.text.length - SelLength;
  }
  return pos;
}

export function initRepoEditor() {
  initEditorForm();

  $('.js-quick-pull-choice-option').on('change', function () {
    if ($(this).val() === 'commit-to-new-branch') {
      showElem('.quick-pull-branch-name');
      document.querySelector('.quick-pull-branch-name input').required = true;
    } else {
      hideElem('.quick-pull-branch-name');
      document.querySelector('.quick-pull-branch-name input').required = false;
    }
    $('#commit-button').text(this.getAttribute('button_text'));
  });

  const joinTreePath = ($fileNameEl) => {
    const parts = [];
    $('.breadcrumb span.section').each(function () {
      const $element = $(this);
      if ($element.find('a').length) {
        parts.push($element.find('a').text());
      } else {
        parts.push($element.text());
      }
    });
    if ($fileNameEl.val()) parts.push($fileNameEl.val());
    $('#tree_path').val(parts.join('/'));
  };

  const $editFilename = $('#file-name');
  $editFilename.on('input', function () {
    const parts = $(this).val().split('/');

    if (parts.length > 1) {
      for (let i = 0; i < parts.length; ++i) {
        const value = parts[i];
        if (i < parts.length - 1) {
          if (value.length) {
            $(`<span class="section"><a href="#">${htmlEscape(value)}</a></span>`).insertBefore($(this));
            $('<div class="breadcrumb-divider">/</div>').insertBefore($(this));
          }
        } else {
          $(this).val(value);
        }
        this.setSelectionRange(0, 0);
      }
    }

    joinTreePath($(this));
  });

  $editFilename.on('keydown', function (e) {
    const $section = $('.breadcrumb span.section');

    // Jump back to last directory once the filename is empty
    if (e.code === 'Backspace' && getCursorPosition($(this)) === 0 && $section.length > 0) {
      e.preventDefault();
      const $divider = $('.breadcrumb .breadcrumb-divider');
      const value = $section.last().find('a').text();
      $(this).val(value + $(this).val());
      this.setSelectionRange(value.length, value.length);
      $section.last().remove();
      $divider.last().remove();
      joinTreePath($(this));
    }
  });

  const $editArea = $('.repository.editor textarea#edit_area');
  if (!$editArea.length) return;

  (async () => {
    const editor = await createCodeEditor($editArea[0], $editFilename[0]);

    // Using events from https://github.com/codedance/jquery.AreYouSure#advanced-usage
    // to enable or disable the commit button
    const commitButton = document.getElementById('commit-button');
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
