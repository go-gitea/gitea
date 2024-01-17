import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import {createCodeEditor} from './codeeditor.js';
import {hideElem, showElem} from '../utils/dom.js';
import {initMarkupContent} from '../markup/content.js';
import {attachRefIssueContextPopup} from './contextpopup.js';

const {csrfToken} = window.config;

function initEditPreviewTab($form) {
  const $tabMenu = $form.find('.tabular.menu');
  $tabMenu.find('.item').tab();
  const $previewTab = $tabMenu.find(`.item[data-tab="${$tabMenu.data('preview')}"]`);
  if ($previewTab.length) {
    $previewTab.on('click', function () {
      const $this = $(this);
      let context = `${$this.data('context')}/`;
      const mode = $this.data('markup-mode') || 'comment';
      const treePathEl = $form.find('input#tree_path');
      if (treePathEl.length > 0) {
        context += treePathEl.val();
      }
      context = context.substring(0, context.lastIndexOf('/'));
      $.post($this.data('url'), {
        _csrf: csrfToken,
        mode,
        context,
        text: $form.find(`.tab[data-tab="${$tabMenu.data('write')}"] textarea`).val(),
        file_path: treePathEl.val(),
      }, (data) => {
        const $previewPanel = $form.find(`.tab[data-tab="${$tabMenu.data('preview')}"]`);
        renderPreviewPanelContent($previewPanel, data);
      });
    });
  }
}

function initEditDiffTab($form) {
  const $tabMenu = $form.find('.tabular.menu');
  $tabMenu.find('.item').tab();
  $tabMenu.find(`.item[data-tab="${$tabMenu.data('diff')}"]`).on('click', function () {
    const $this = $(this);
    $.post($this.data('url'), {
      _csrf: csrfToken,
      context: $this.data('context'),
      content: $form.find(`.tab[data-tab="${$tabMenu.data('write')}"] textarea`).val(),
    }, (data) => {
      const $diffPreviewPanel = $form.find(`.tab[data-tab="${$tabMenu.data('diff')}"]`);
      $diffPreviewPanel.html(data);
    });
  });
}

function initEditorForm() {
  if ($('.repository .edit.form').length === 0) {
    return;
  }

  initEditPreviewTab($('.repository .edit.form'));
  initEditDiffTab($('.repository .edit.form'));
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
      showElem($('.quick-pull-branch-name'));
      $('.quick-pull-branch-name input').prop('required', true);
    } else {
      hideElem($('.quick-pull-branch-name'));
      $('.quick-pull-branch-name input').prop('required', false);
    }
    $('#commit-button').text($(this).attr('button_text'));
  });

  const joinTreePath = ($fileNameEl) => {
    const parts = [];
    $('.breadcrumb span.section').each(function () {
      const element = $(this);
      if (element.find('a').length) {
        parts.push(element.find('a').text());
      } else {
        parts.push(element.text());
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
    const $commitButton = $('#commit-button');
    const $editForm = $('.ui.edit.form');
    const dirtyFileClass = 'dirty-file';

    // Disabling the button at the start
    if ($('input[name="page_has_posted"]').val() !== 'true') {
      $commitButton.prop('disabled', true);
    }

    // Registering a custom listener for the file path and the file content
    $editForm.areYouSure({
      silent: true,
      dirtyClass: dirtyFileClass,
      fieldSelector: ':input:not(.commit-form-wrapper :input)',
      change() {
        const dirty = $(this).hasClass(dirtyFileClass);
        $commitButton.prop('disabled', !dirty);
      },
    });

    // Update the editor from query params, if available,
    // only after the dirtyFileClass initialization
    const params = new URLSearchParams(window.location.search);
    const value = params.get('value');
    if (value) {
      editor.setValue(value);
    }

    $commitButton.on('click', (event) => {
      // A modal which asks if an empty file should be committed
      if ($editArea.val().length === 0) {
        $('#edit-empty-content-modal').modal({
          onApprove() {
            $('.edit.form').trigger('submit');
          },
        }).modal('show');
        event.preventDefault();
      }
    });
  })();
}

export function renderPreviewPanelContent($panelPreviewer, data) {
  $panelPreviewer.html(data);
  initMarkupContent();

  const refIssues = $panelPreviewer.find('p .ref-issue');
  attachRefIssueContextPopup(refIssues);
}
