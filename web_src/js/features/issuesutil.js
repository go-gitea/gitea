import {htmlEscape} from 'escape-goat';
import Vue from 'vue';
import attachTribute from './tribute.js';
import createColorPicker from './colorpicker.js';
import createDropzone from './dropzone.js';
import renderMarkdownContent from '../markdown/content.js';
import {createCodeEditor} from './codeeditor.js';


const {AppSubUrl, csrf} = window.config;
const commentMDEditors = {};

let previewFileModes;
let autoSimpleMDE;

function insertAtCursor(field, value) {
  if (field.selectionStart || field.selectionStart === 0) {
    const startPos = field.selectionStart;
    const endPos = field.selectionEnd;
    field.value = field.value.substring(0, startPos) + value + field.value.substring(endPos, field.value.length);
    field.selectionStart = startPos + value.length;
    field.selectionEnd = startPos + value.length;
  } else {
    field.value += value;
  }
}

function replaceAndKeepCursor(field, oldval, newval) {
  if (field.selectionStart || field.selectionStart === 0) {
    const startPos = field.selectionStart;
    const endPos = field.selectionEnd;
    field.value = field.value.replace(oldval, newval);
    field.selectionStart = startPos + newval.length - oldval.length;
    field.selectionEnd = endPos + newval.length - oldval.length;
  } else {
    field.value = field.value.replace(oldval, newval);
  }
}

function getPastedImages(e) {
  if (!e.clipboardData) return [];

  const files = [];
  for (const item of e.clipboardData.items || []) {
    if (!item.type || !item.type.startsWith('image/')) continue;
    files.push(item.getAsFile());
  }

  if (files.length) {
    e.preventDefault();
    e.stopPropagation();
  }
  return files;
}

async function uploadFile(file) {
  const formData = new FormData();
  formData.append('file', file, file.name);

  const res = await fetch($('#dropzone').data('upload-url'), {
    method: 'POST',
    headers: {'X-Csrf-Token': csrf},
    body: formData,
  });
  return await res.json();
}

function initImagePaste(target) {
  target.each(function () {
    const field = this;
    field.addEventListener('paste', async (e) => {
      for (const img of getPastedImages(e)) {
        const name = img.name.substr(0, img.name.lastIndexOf('.'));
        insertAtCursor(field, `![${name}]()`);
        const data = await uploadFile(img);
        replaceAndKeepCursor(field, `![${name}]()`, `![${name}](${AppSubUrl}/attachments/${data.uuid})`);
        const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
        $('.files').append(input);
      }
    }, false);
  });
}

function initReactionSelector(parent) {
  let reactions = '';
  if (!parent) {
    parent = $(document);
    reactions = '.reactions > ';
  }

  parent.find(`${reactions}a.label`).popup({position: 'bottom left', metadata: {content: 'title', title: 'none'}});

  parent.find(`.select-reaction > .menu > .item, ${reactions}a.label`).on('click', function (e) {
    const vm = this;
    e.preventDefault();

    if ($(this).hasClass('disabled')) return;

    const actionURL = $(this).hasClass('item') ? $(this).closest('.select-reaction').data('action-url') : $(this).data('action-url');
    const url = `${actionURL}/${$(this).hasClass('blue') ? 'unreact' : 'react'}`;
    $.ajax({
      type: 'POST',
      url,
      data: {
        _csrf: csrf,
        content: $(this).data('content')
      }
    }).done((resp) => {
      if (resp && (resp.html || resp.empty)) {
        const content = $(vm).closest('.content');
        let react = content.find('.segment.reactions');
        if ((!resp.empty || resp.html === '') && react.length > 0) {
          react.remove();
        }
        if (!resp.empty) {
          react = $('<div class="ui attached segment reactions"></div>');
          const attachments = content.find('.segment.bottom:first');
          if (attachments.length > 0) {
            react.insertBefore(attachments);
          } else {
            react.appendTo(content);
          }
          react.html(resp.html);
          react.find('.dropdown').dropdown();
          initReactionSelector(react);
        }
      }
    });
  });
}

function initSimpleMDEImagePaste(simplemde, files) {
  simplemde.codemirror.on('paste', async (_, e) => {
    for (const img of getPastedImages(e)) {
      const name = img.name.substr(0, img.name.lastIndexOf('.'));
      const data = await uploadFile(img);
      const pos = simplemde.codemirror.getCursor();
      simplemde.codemirror.replaceRange(`![${name}](${AppSubUrl}/attachments/${data.uuid})`, pos);
      const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
      files.append(input);
    }
  });
}

export async function initEditor() {
  $('.js-quick-pull-choice-option').on('change', function () {
    if ($(this).val() === 'commit-to-new-branch') {
      $('.quick-pull-branch-name').show();
      $('.quick-pull-branch-name input').prop('required', true);
    } else {
      $('.quick-pull-branch-name').hide();
      $('.quick-pull-branch-name input').prop('required', false);
    }
    $('#commit-button').text($(this).attr('button_text'));
  });

  const $editFilename = $('#file-name');
  $editFilename.on('keyup', function (e) {
    const $section = $('.breadcrumb span.section');
    const $divider = $('.breadcrumb div.divider');
    let value;
    let parts;

    if (e.keyCode === 8) {
      if ($(this).getCursorPosition() === 0) {
        if ($section.length > 0) {
          value = $section.last().find('a').text();
          $(this).val(value + $(this).val());
          $(this)[0].setSelectionRange(value.length, value.length);
          $section.last().remove();
          $divider.last().remove();
        }
      }
    }
    if (e.keyCode === 191) {
      parts = $(this).val().split('/');
      for (let i = 0; i < parts.length; ++i) {
        value = parts[i];
        if (i < parts.length - 1) {
          if (value.length) {
            $(`<span class="section"><a href="#">${value}</a></span>`).insertBefore($(this));
            $('<div class="divider"> / </div>').insertBefore($(this));
          }
        } else {
          $(this).val(value);
        }
        $(this)[0].setSelectionRange(0, 0);
      }
    }
    parts = [];
    $('.breadcrumb span.section').each(function () {
      const element = $(this);
      if (element.find('a').length) {
        parts.push(element.find('a').text());
      } else {
        parts.push(element.text());
      }
    });
    if ($(this).val()) parts.push($(this).val());
    $('#tree_path').val(parts.join('/'));
  }).trigger('keyup');

  const $editArea = $('.repository.editor textarea#edit_area');
  if (!$editArea.length) return;

  await createCodeEditor($editArea[0], $editFilename[0], previewFileModes);

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
    }
  });

  $commitButton.on('click', (event) => {
    // A modal which asks if an empty file should be committed
    if ($editArea.val().length === 0) {
      $('#edit-empty-content-modal').modal({
        onApprove() {
          $('.edit.form').trigger('submit');
        }
      }).modal('show');
      event.preventDefault();
    }
  });
}

export function updateIssuesMeta(url, action, issueIds, elementId) {
  return new Promise(((resolve) => {
    $.ajax({
      type: 'POST',
      url,
      data: {
        _csrf: csrf,
        action,
        issue_ids: issueIds,
        id: elementId,
      },
      success: resolve
    }).done((_data) => {
      const projectID = $('[data-project]').data('project');
      if (projectID === elementId) {
        // same project, don't remove
      } else {
        if (url.endsWith('/projects')) {
          $(`.board-card[data-issueid=${issueIds}]`).remove();
          $('#current-card-details').hide();
          // different project, removing
        }
      }
    });
  }));
}

export function initRepoStatusChecker() {
  const migrating = $('#repo_migrating');
  $('#repo_migrating_failed').hide();
  $('#repo_migrating_failed_image').hide();
  if (migrating) {
    const task = migrating.attr('task');
    if (typeof task === 'undefined') {
      return;
    }
    $.ajax({
      type: 'GET',
      url: `${AppSubUrl}/user/task/${task}`,
      data: {
        _csrf: csrf,
      },
      complete(xhr) {
        if (xhr.status === 200) {
          if (xhr.responseJSON) {
            if (xhr.responseJSON.status === 4) {
              window.location.reload();
              return;
            } else if (xhr.responseJSON.status === 3) {
              $('#repo_migrating_progress').hide();
              $('#repo_migrating').hide();
              $('#repo_migrating_failed').show();
              $('#repo_migrating_failed_image').show();
              $('#repo_migrating_failed_error').text(xhr.responseJSON.err);
              return;
            }
            setTimeout(() => {
              initRepoStatusChecker();
            }, 2000);
            return;
          }
        }
        $('#repo_migrating_progress').hide();
        $('#repo_migrating').hide();
        $('#repo_migrating_failed').show();
        $('#repo_migrating_failed_image').show();
      }
    });
  }
}

export function buttonsClickOnEnter() {
  $('.ui.button').on('keypress', function (e) {
    if (e.keyCode === 13 || e.keyCode === 32) { // enter key or space bar
      $(this).trigger('click');
    }
  });
}

export function initCommentPreviewTab($form) {
  const $tabMenu = $form.find('.tabular.menu');
  $tabMenu.find('.item').tab();
  $tabMenu.find(`.item[data-tab="${$tabMenu.data('preview')}"]`).on('click', function () {
    const $this = $(this);
    $.post($this.data('url'), {
      _csrf: csrf,
      mode: 'comment',
      context: $this.data('context'),
      text: $form.find(`.tab[data-tab="${$tabMenu.data('write')}"] textarea`).val()
    }, (data) => {
      const $previewPanel = $form.find(`.tab[data-tab="${$tabMenu.data('preview')}"]`);
      $previewPanel.html(data);
      renderMarkdownContent();
    });
  });

  buttonsClickOnEnter();
}

export function initEditPreviewTab($form) {
  const $tabMenu = $form.find('.tabular.menu');
  $tabMenu.find('.item').tab();
  const $previewTab = $tabMenu.find(`.item[data-tab="${$tabMenu.data('preview')}"]`);
  if ($previewTab.length) {
    previewFileModes = $previewTab.data('preview-file-modes').split(',');
    $previewTab.on('click', function () {
      const $this = $(this);
      let context = `${$this.data('context')}/`;
      const mode = $this.data('markdown-mode') || 'comment';
      const treePathEl = $form.find('input#tree_path');
      if (treePathEl.length > 0) {
        context += treePathEl.val();
      }
      context = context.substring(0, context.lastIndexOf('/'));
      $.post($this.data('url'), {
        _csrf: csrf,
        mode,
        context,
        text: $form.find(`.tab[data-tab="${$tabMenu.data('write')}"] textarea`).val()
      }, (data) => {
        const $previewPanel = $form.find(`.tab[data-tab="${$tabMenu.data('preview')}"]`);
        $previewPanel.html(data);
        renderMarkdownContent();
      });
    });
  }
}

function initIssueComments() {
  if ($('.repository.view.issue .timeline').length === 0) return;

  $('.re-request-review').on('click', function (event) {
    const url = $(this).data('update-url');
    const issueId = $(this).data('issue-id');
    const id = $(this).data('id');
    const isChecked = $(this).hasClass('checked');

    event.preventDefault();
    updateIssuesMeta(
      url,
      isChecked ? 'detach' : 'attach',
      issueId,
      id,
    ).then(reload);
    return false;
  });

  $(document).on('click', (event) => {
    const urlTarget = $(':target');
    if (urlTarget.length === 0) return;

    const urlTargetId = urlTarget.attr('id');
    if (!urlTargetId) return;
    if (!/^(issue|pull)(comment)?-\d+$/.test(urlTargetId)) return;

    const $target = $(event.target);

    if ($target.closest(`#${urlTargetId}`).length === 0) {
      const scrollPosition = $(window).scrollTop();
      window.location.hash = '';
      $(window).scrollTop(scrollPosition);
      window.history.pushState(null, null, ' ');
    }
  });
}

export function initLabelEdit() {
// Create label
  const $newLabelPanel = $('.new-label.segment');
  $('.new-label.button').on('click', () => {
    $newLabelPanel.show();
  });
  $('.new-label.segment .cancel').on('click', () => {
    $newLabelPanel.hide();
  });

  createColorPicker($('.color-picker'));

  $('.precolors .color').on('click', function () {
    const color_hex = $(this).data('color-hex');
    $('.color-picker').val(color_hex);
    $('.minicolors-swatch-color').css('background-color', color_hex);
  });
  $('.edit-label-button').on('click', function () {
    $('.color-picker').minicolors('value', $(this).data('color'));
    $('#label-modal-id').val($(this).data('id'));
    $('.edit-label .new-label-input').val($(this).data('title'));
    $('.edit-label .new-label-desc-input').val($(this).data('description'));
    $('.edit-label .color-picker').val($(this).data('color'));
    $('.minicolors-swatch-color').css('background-color', $(this).data('color'));
    $('.edit-label.modal').modal({
      onApprove() {
        $('.edit-label.form').trigger('submit');
      }
    }).modal('show');
    return false;
  });
}


export function reload() {
  if ($('#current-card-details').length > 0) {
    reloadIssuesActions();
  } else {
    window.location.reload();
  }
}

// subscribe-repo
// toggle_stopwatch_form"
// cancel_stopwatch_form"
// add_time_manual_form
// addDependencyForm
function initAjaxForms(formId) {
  $(formId).submit(function(e) {
    e.preventDefault(); // avoid to execute the actual submit of the form.

    const form = $(this);
    const url = form.attr('action');

    $.ajax({
      type: 'POST',
      url,
      data: form.serialize(), // serializes the form's elements.
      success(_data) {
        $('#lock').modal('hide');
        $('#lock').remove();
        reloadIssuesActions();
      }
    });
  });
}

export function initIssueList() {
  const repolink = $('#repolink').val();
  const repoId = $('#repoId').val();
  const crossRepoSearch = $('#crossRepoSearch').val();
  const tp = $('#type').val();
  let issueSearchUrl = `${AppSubUrl}/api/v1/repos/${repolink}/issues?q={query}&type=${tp}`;
  if (crossRepoSearch === 'true') {
    issueSearchUrl = `${AppSubUrl}/api/v1/repos/issues/search?q={query}&priority_repo_id=${repoId}&type=${tp}`;
  }
  $('#new-dependency-drop-list').dropdown({
    apiSettings: {
      url: issueSearchUrl,
      onResponse(response) {
        const filteredResponse = {success: true, results: []};
        const currIssueId = $('#new-dependency-drop-list').data('issue-id');
        // Parse the response from the api to work with our dropdown
        $.each(response, (_i, issue) => {
          // Don't list current issue in the dependency list.
          if (issue.id === currIssueId) {
            return;
          }
          filteredResponse.results.push({
            name: `#${issue.number} ${htmlEscape(issue.title)}<div class="text small dont-break-out">${htmlEscape(issue.repository.full_name)}</div>`,
            value: issue.id
          });
        });
        return filteredResponse;
      },
      cache: false,
    },

    fullTextSearch: true
  });

  $('.menu a.label-filter-item').each(function () {
    $(this).on('click', function (e) {
      if (e.altKey) {
        e.preventDefault();

        const href = $(this).attr('href');
        const id = $(this).data('label-id');

        const regStr = `labels=(-?[0-9]+%2c)*(${id})(%2c-?[0-9]+)*&`;
        const newStr = 'labels=$1-$2$3&';

        window.location = href.replace(new RegExp(regStr), newStr);
      }
    });
  });

  $('.menu .ui.dropdown.label-filter').on('keydown', (e) => {
    if (e.altKey && e.keyCode === 13) {
      const selectedItems = $('.menu .ui.dropdown.label-filter .menu .item.selected');

      if (selectedItems.length > 0) {
        const item = $(selectedItems[0]);

        const href = item.attr('href');
        const id = item.data('label-id');

        const regStr = `labels=(-?[0-9]+%2c)*(${id})(%2c-?[0-9]+)*&`;
        const newStr = 'labels=$1-$2$3&';

        window.location = href.replace(new RegExp(regStr), newStr);
      }
    }
  });
}

export async function reloadIssuesActions(data) {
  if ($('#current-card-details').length === 0) {
    window.location.reload();
  }
  if (!data || !data.url) {
    data = $('body').data();
    if (!data.url) {
      data.url = `${window.location.href}/sidebar/true`;
    }
  } else {
    $('body').data('url', data.url);
  }
  fetch(data.url, {
    method: 'GET',
    headers: {'X-Csrf-Token': csrf, 'Content-Type': 'text/html'}
  })
    .then((res) => {
      return res.text();
    })

    .then((html) => {
      $('#current-card-details').html(html);
      $('#current-card-details').removeClass('hide');
      initCommentForm(true);
      initIssueList();
      initAjaxForms('#subscribe-repo');
      initAjaxForms('#toggle_stopwatch_form');
      initAjaxForms('#cancel_stopwatch_form');
      initAjaxForms('#add_time_manual_form');
      initAjaxForms('#addDependencyForm');
      initAjaxForms('#lock-form');
      initAjaxForms('#removeDependencyForm');
      $('#lock-dropdown').dropdown('show');
      $('#lock').modal('hide');
      $('.show-modal.button').on('click', function() {
        $($(this).data('modal')).modal('show');
      });
    });
}

export async function initRepository() {
  $('body').keyup((e) => {
    if (e.keyCode === 27) {
      $('#current-card-details').hide();
      $('#current-card-details').html('');
      $('#current-card-details-input').html('');
    }
  });

  if ($('.repository').length === 0) {
    return;
  }

  function initFilterSearchDropdown(selector) {
    const $dropdown = $(selector);
    $dropdown.dropdown({
      fullTextSearch: true,
      selectOnKeydown: false,
      onChange(_text, _value, $choice) {
        if ($choice.data('url')) {
          window.location.href = $choice.data('url');
        }
      },
      message: {noResults: $dropdown.data('no-results')}
    });
  }

  // File list and commits
  if ($('.repository.file.list').length > 0 || ('.repository.commits').length > 0) {
    initFilterBranchTagDropdown('.choose.reference .dropdown');
  }

  // Wiki
  if ($('.repository.wiki.view').length > 0) {
    initFilterSearchDropdown('.choose.page .dropdown');
  }

  // Options
  if ($('.repository.settings.options').length > 0) {
    // Enable or select internal/external wiki system and issue tracker.
    $('.enable-system').on('change', function () {
      if (this.checked) {
        $($(this).data('target')).removeClass('disabled');
        if (!$(this).data('context')) $($(this).data('context')).addClass('disabled');
      } else {
        $($(this).data('target')).addClass('disabled');
        if (!$(this).data('context')) $($(this).data('context')).removeClass('disabled');
      }
    });
    $('.enable-system-radio').on('change', function () {
      if (this.value === 'false') {
        $($(this).data('target')).addClass('disabled');
        if (typeof $(this).data('context') !== 'undefined') $($(this).data('context')).removeClass('disabled');
      } else if (this.value === 'true') {
        $($(this).data('target')).removeClass('disabled');
        if (typeof $(this).data('context') !== 'undefined') $($(this).data('context')).addClass('disabled');
      }
    });
  }

  // Labels
  if ($('.repository.labels').length > 0) {
    initLabelEdit();
  }

  // Milestones
  if ($('.repository.new.milestone').length > 0) {
    $('#clear-date').on('click', () => {
      $('#deadline').val('');
      return false;
    });
  }

  // Repo Creation
  if ($('.repository.new.repo').length > 0) {
    $('input[name="gitignores"], input[name="license"]').on('change', () => {
      const gitignores = $('input[name="gitignores"]').val();
      const license = $('input[name="license"]').val();
      if (gitignores || license) {
        $('input[name="auto_init"]').prop('checked', true);
      }
    });
  }

  // Issues
  if ($('.repository.view.issue').length > 0) {
    // Edit issue title
    const $issueTitle = $('#issue-title');
    const $editInput = $('#edit-title-input input');
    const editTitleToggle = function () {
      $issueTitle.toggle();
      $('.not-in-edit').toggle();
      $('#edit-title-input').toggle();
      $('#pull-desc').toggle();
      $('#pull-desc-edit').toggle();
      $('.in-edit').toggle();
      $('#issue-title-wrapper').toggleClass('edit-active');
      $editInput.focus();
      return false;
    };

    const changeBranchSelect = function () {
      const selectionTextField = $('#pull-target-branch');

      const baseName = selectionTextField.data('basename');
      const branchNameNew = $(this).data('branch');
      const branchNameOld = selectionTextField.data('branch');

      // Replace branch name to keep translation from HTML template
      selectionTextField.html(selectionTextField.html().replace(
        `${baseName}:${branchNameOld}`,
        `${baseName}:${branchNameNew}`
      ));
      selectionTextField.data('branch', branchNameNew); // update branch name in setting
    };
    $('#branch-select > .item').on('click', changeBranchSelect);

    $('#edit-title').on('click', editTitleToggle);
    $('#cancel-edit-title').on('click', editTitleToggle);
    $('#save-edit-title').on('click', editTitleToggle).on('click', function () {
      const pullrequest_targetbranch_change = function (update_url) {
        const targetBranch = $('#pull-target-branch').data('branch');
        const $branchTarget = $('#branch_target');
        if (targetBranch === $branchTarget.text()) {
          return false;
        }
        $.post(update_url, {
          _csrf: csrf,
          target_branch: targetBranch
        }).done((data) => {
          $branchTarget.text(data.base_branch);
        }).always(() => {
          reload();
        });
      };

      const pullrequest_target_update_url = $(this).data('target-update-url');
      if ($editInput.val().length === 0 || $editInput.val() === $issueTitle.text()) {
        $editInput.val($issueTitle.text());
        pullrequest_targetbranch_change(pullrequest_target_update_url);
      } else {
        $.post($(this).data('update-url'), {
          _csrf: csrf,
          title: $editInput.val()
        }, (data) => {
          $editInput.val(data.title);
          $issueTitle.text(data.title);
          pullrequest_targetbranch_change(pullrequest_target_update_url);
          reload();
        });
      }
      return false;
    });

    // Issue Comments
    initIssueComments();

    // Issue/PR Context Menus
    $('.context-dropdown').dropdown({
      action: 'hide'
    });

    // Quote reply
    $('.quote-reply').on('click', function (event) {
      $(this).closest('.dropdown').find('.menu').toggle('visible');
      const target = $(this).data('target');
      const quote = $(`#comment-${target}`).text().replace(/\n/g, '\n> ');
      const content = `> ${quote}\n\n`;

      let $content;
      if ($(this).hasClass('quote-reply-diff')) {
        const $parent = $(this).closest('.comment-code-cloud');
        $parent.find('button.comment-form-reply').trigger('click');
        $content = $parent.find('[name="content"]');
        if ($content.val() !== '') {
          $content.val(`${$content.val()}\n\n${content}`);
        } else {
          $content.val(`${content}`);
        }
        $content.focus();
      } else if (autoSimpleMDE !== null) {
        if (autoSimpleMDE.value() !== '') {
          autoSimpleMDE.value(`${autoSimpleMDE.value()}\n\n${content}`);
        } else {
          autoSimpleMDE.value(`${content}`);
        }
      }
      event.preventDefault();
    });

    // Edit issue or comment content
    $('.edit-content').on('click', async function (event) {
      $(this).closest('.dropdown').find('.menu').toggle('visible');
      const $segment = $(this).closest('.header').next();
      const $editContentZone = $segment.find('.edit-content-zone');
      const $renderContent = $segment.find('.render-content');
      const $rawContent = $segment.find('.raw-content');
      let $textarea;
      let $simplemde;

      // Setup new form
      if ($editContentZone.html().length === 0) {
        $editContentZone.html($('#edit-content-form').html());
        $textarea = $editContentZone.find('textarea');
        attachTribute($textarea.get(), {mentions: true, emoji: true});

        let dz;
        const $dropzone = $editContentZone.find('.dropzone');
        const $files = $editContentZone.find('.comment-files');
        if ($dropzone.length > 0) {
          $dropzone.data('saved', false);

          const filenameDict = {};
          dz = await createDropzone($dropzone[0], {
            url: $dropzone.data('upload-url'),
            headers: {'X-Csrf-Token': csrf},
            maxFiles: $dropzone.data('max-file'),
            maxFilesize: $dropzone.data('max-size'),
            acceptedFiles: (['*/*', ''].includes($dropzone.data('accepts'))) ? null : $dropzone.data('accepts'),
            addRemoveLinks: true,
            dictDefaultMessage: $dropzone.data('default-message'),
            dictInvalidFileType: $dropzone.data('invalid-input-type'),
            dictFileTooBig: $dropzone.data('file-too-big'),
            dictRemoveFile: $dropzone.data('remove-file'),
            timeout: 0,
            init() {
              this.on('success', (file, data) => {
                filenameDict[file.name] = {
                  uuid: data.uuid,
                  submitted: false
                };
                const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
                $files.append(input);
              });
              this.on('removedfile', (file) => {
                if (!(file.name in filenameDict)) {
                  return;
                }
                $(`#${filenameDict[file.name].uuid}`).remove();
                if ($dropzone.data('remove-url') && !filenameDict[file.name].submitted) {
                  $.post($dropzone.data('remove-url'), {
                    file: filenameDict[file.name].uuid,
                    _csrf: csrf,
                  });
                }
              });
              this.on('submit', () => {
                $.each(filenameDict, (name) => {
                  filenameDict[name].submitted = true;
                });
              });
              this.on('reload', () => {
                $.getJSON($editContentZone.data('attachment-url'), (data) => {
                  dz.removeAllFiles(true);
                  $files.empty();
                  $.each(data, function () {
                    const imgSrc = `${$dropzone.data('link-url')}/${this.uuid}`;
                    dz.emit('addedfile', this);
                    dz.emit('thumbnail', this, imgSrc);
                    dz.emit('complete', this);
                    dz.files.push(this);
                    filenameDict[this.name] = {
                      submitted: true,
                      uuid: this.uuid
                    };
                    $dropzone.find(`img[src='${imgSrc}']`).css('max-width', '100%');
                    const input = $(`<input id="${this.uuid}" name="files" type="hidden">`).val(this.uuid);
                    $files.append(input);
                  });
                });
              });
            }
          });
          dz.emit('reload');
        }
        // Give new write/preview data-tab name to distinguish from others
        const $editContentForm = $editContentZone.find('.ui.comment.form');
        const $tabMenu = $editContentForm.find('.tabular.menu');
        $tabMenu.attr('data-write', $editContentZone.data('write'));
        $tabMenu.attr('data-preview', $editContentZone.data('preview'));
        $tabMenu.find('.write.item').attr('data-tab', $editContentZone.data('write'));
        $tabMenu.find('.preview.item').attr('data-tab', $editContentZone.data('preview'));
        $editContentForm.find('.write').attr('data-tab', $editContentZone.data('write'));
        $editContentForm.find('.preview').attr('data-tab', $editContentZone.data('preview'));
        $simplemde = setCommentSimpleMDE($textarea);
        commentMDEditors[$editContentZone.data('write')] = $simplemde;
        initCommentPreviewTab($editContentForm);
        initSimpleMDEImagePaste($simplemde, $files);

        $editContentZone.find('.cancel.button').on('click', () => {
          $renderContent.show();
          $editContentZone.hide();
          if (dz) {
            dz.emit('reload');
          }
        });
        $editContentZone.find('.save.button').on('click', () => {
          $renderContent.show();
          $editContentZone.hide();
          const $attachments = $files.find('[name=files]').map(function () {
            return $(this).val();
          }).get();
          $.post($editContentZone.data('update-url'), {
            _csrf: csrf,
            content: $textarea.val(),
            context: $editContentZone.data('context'),
            files: $attachments
          }, (data) => {
            if (data.length === 0 || data.content.length === 0) {
              $renderContent.html($('#no-content').html());
            } else {
              $renderContent.html(data.content);
            }
            const $content = $segment;
            if (!$content.find('.dropzone-attachments').length) {
              if (data.attachments !== '') {
                $content.append(`
                  <div class="dropzone-attachments">
                    <div class="ui clearing divider"></div>
                    <div class="ui middle aligned padded grid">
                    </div>
                  </div>
                `);
                $content.find('.dropzone-attachments .grid').html(data.attachments);
              }
            } else if (data.attachments === '') {
              $content.find('.dropzone-attachments').remove();
            } else {
              $content.find('.dropzone-attachments .grid').html(data.attachments);
            }
            if (dz) {
              dz.emit('submit');
              dz.emit('reload');
            }
            renderMarkdownContent();
          });
        });
      } else {
        $textarea = $segment.find('textarea');
        $simplemde = commentMDEditors[$editContentZone.data('write')];
      }

      // Show write/preview tab and copy raw content as needed
      $editContentZone.show();
      $renderContent.hide();
      if ($textarea.val().length === 0) {
        $textarea.val($rawContent.text());
        $simplemde.value($rawContent.text());
      }
      $textarea.focus();
      $simplemde.codemirror.focus();
      event.preventDefault();
    });

    // Delete comment
    $('.delete-comment').on('click', function () {
      const $this = $(this);
      if (window.confirm($this.data('locale'))) {
        $.post($this.data('url'), {
          _csrf: csrf
        }).done(() => {
          $(`#${$this.data('comment-id')}`).remove();
        });
      }
      return false;
    });

    // Change status
    const $statusButton = $('#status-button');
    $('#comment-form .edit_area').on('keyup', function () {
      if ($(this).val().length === 0) {
        $statusButton.text($statusButton.data('status'));
      } else {
        $statusButton.text($statusButton.data('status-and-comment'));
      }
    });
    $statusButton.on('click', () => {
      $('#status').val($statusButton.data('status-val'));
      $('#comment-form').trigger('submit');
    });

    // Pull Request merge button
    const $mergeButton = $('.merge-button > button');
    $mergeButton.on('click', function (e) {
      e.preventDefault();
      $(`.${$(this).data('do')}-fields`).show();
      $(this).parent().hide();
    });
    $('.merge-button > .dropdown').dropdown({
      onChange(_text, _value, $choice) {
        if ($choice.data('do')) {
          $mergeButton.find('.button-text').text($choice.text());
          $mergeButton.data('do', $choice.data('do'));
        }
      }
    });
    $('.merge-cancel').on('click', function (e) {
      e.preventDefault();
      $(this).closest('.form').hide();
      $mergeButton.parent().show();
    });
    initReactionSelector();
  }

  // Quick start and repository home
  $('#repo-clone-ssh').on('click', function () {
    $('.clone-url').text($(this).data('link'));
    $('#repo-clone-url').val($(this).data('link'));
    $(this).addClass('blue');
    $('#repo-clone-https').removeClass('blue');
    localStorage.setItem('repo-clone-protocol', 'ssh');
  });
  $('#repo-clone-https').on('click', function () {
    $('.clone-url').text($(this).data('link'));
    $('#repo-clone-url').val($(this).data('link'));
    $(this).addClass('blue');
    if ($('#repo-clone-ssh').length > 0) {
      $('#repo-clone-ssh').removeClass('blue');
      localStorage.setItem('repo-clone-protocol', 'https');
    }
  });
  $('#repo-clone-url').on('click', function () {
    $(this).select();
  });

  // Pull request
  const $repoComparePull = $('.repository.compare.pull');
  if ($repoComparePull.length > 0) {
    initFilterSearchDropdown('.choose.branch .dropdown');
    // show pull request form
    $repoComparePull.find('button.show-form').on('click', function (e) {
      e.preventDefault();
      $repoComparePull.find('.pullrequest-form').show();
      autoSimpleMDE.codemirror.refresh();
      $(this).parent().hide();
    });
  }

  // Branches
  if ($('.repository.settings.branches').length > 0) {
    initFilterSearchDropdown('.protected-branches .dropdown');
    $('.enable-protection, .enable-whitelist, .enable-statuscheck').on('change', function () {
      if (this.checked) {
        $($(this).data('target')).removeClass('disabled');
      } else {
        $($(this).data('target')).addClass('disabled');
      }
    });
    $('.disable-whitelist').on('change', function () {
      if (this.checked) {
        $($(this).data('target')).addClass('disabled');
      }
    });
  }

  // Language stats
  if ($('.language-stats').length > 0) {
    $('.language-stats').on('click', (e) => {
      e.preventDefault();
      $('.language-stats-details, .repository-menu').slideToggle();
    });
  }
}

export function initCommentForm(skipCommentForm) {
  if (!skipCommentForm) {
    if ($('.comment.form').length === 0) {
      return;
    }

    autoSimpleMDE = setCommentSimpleMDE(
      $('.comment.form textarea:not(.review-textarea)')
    );
    initCommentPreviewTab($('.comment.form'));
    initImagePaste($('.comment.form textarea'));
  }
  initBranchSelector();

  // Init labels and assignees
  initListSubmits('select-label', 'labels');
  initListSubmits('select-milestone', 'milestone');
  initListSubmits('select-project', 'project');
  initListSubmits('select-assignees', 'assignees');
  initListSubmits('select-assignees-modify', 'assignees');
  initListSubmits('select-reviewers-modify', 'assignees');

  // Milestone, Assignee, Project
  selectItem('.select-project', '#project_id');
  selectItem('.select-milestone', '#milestone_id');
  selectItem('.select-assignee', '#assignee_id');
  selectItem('.select-reviewer', '#reviewer_id');
}

export function setCommentSimpleMDE($editArea) {
  const simplemde = new SimpleMDE({
    autoDownloadFontAwesome: false,
    element: $editArea[0],
    forceSync: true,
    renderingConfig: {
      singleLineBreaks: false
    },
    indentWithTabs: false,
    tabSize: 4,
    spellChecker: false,
    toolbar: ['bold', 'italic', 'strikethrough', '|',
      'heading-1', 'heading-2', 'heading-3', 'heading-bigger', 'heading-smaller', '|',
      'code', 'quote', '|', {
        name: 'checkbox-empty',
        action(e) {
          const cm = e.codemirror;
          cm.replaceSelection(`\n- [ ] ${cm.getSelection()}`);
          cm.focus();
        },
        className: 'fa fa-square-o',
        title: 'Add Checkbox (empty)',
      },
      {
        name: 'checkbox-checked',
        action(e) {
          const cm = e.codemirror;
          cm.replaceSelection(`\n- [x] ${cm.getSelection()}`);
          cm.focus();
        },
        className: 'fa fa-check-square-o',
        title: 'Add Checkbox (checked)',
      }, '|',
      'unordered-list', 'ordered-list', '|',
      'link', 'image', 'table', 'horizontal-rule', '|',
      'clean-block', '|',
      {
        name: 'revert-to-textarea',
        action(e) {
          e.toTextArea();
        },
        className: 'fa fa-file',
        title: 'Revert to simple textarea',
      },
    ]
  });
  $(simplemde.codemirror.getInputField()).addClass('js-quick-submit');
  simplemde.codemirror.setOption('extraKeys', {
    Enter: () => {
      const tributeContainer = document.querySelector('.tribute-container');
      if (!tributeContainer || tributeContainer.style.display === 'none') {
        return CodeMirror.Pass;
      }
    },
    Backspace: (cm) => {
      if (cm.getInputField().trigger) {
        cm.getInputField().trigger('input');
      }
      cm.execCommand('delCharBefore');
    }
  });
  attachTribute(simplemde.codemirror.getInputField(), {mentions: true, emoji: true});
  return simplemde;
}

function selectItem(select_id, input_id) {
  const $menu = $(`${select_id} .menu`);
  const $list = $(`.ui${select_id}.list`);
  const hasUpdateAction = $menu.data('action') === 'update';

  $menu.find('.item:not(.no-select)').on('click', function () {
    $(this).parent().find('.item').each(function () {
      $(this).removeClass('selected active');
    });

    $(this).addClass('selected active');
    if (hasUpdateAction) {
      updateIssuesMeta(
        $menu.data('update-url'),
        '',
        $menu.data('issue-id'),
        $(this).data('id'),
      ).then(reload);
    }
    switch (input_id) {
      case '#milestone_id':
        $list.find('.selected').html(`<a class="item" href=${$(this).data('href')}>${
          htmlEscape($(this).text())}</a>`);
        break;
      case '#project_id':
        $list.find('.selected').html(`<a class="item" href=${$(this).data('href')}>${
          htmlEscape($(this).text())}</a>`);
        break;
      case '#assignee_id':
        $list.find('.selected').html(`<a class="item" href=${$(this).data('href')}>` +
                        `<img class="ui avatar image" src=${$(this).data('avatar')}>${
                          htmlEscape($(this).text())}</a>`);
    }
    $(`.ui${select_id}.list .no-select`).addClass('hide');
    $(input_id).val($(this).data('id'));
  });
  $menu.find('.no-select.item').on('click', function () {
    $(this).parent().find('.item:not(.no-select)').each(function () {
      $(this).removeClass('selected active');
    });

    if (hasUpdateAction) {
      updateIssuesMeta(
        $menu.data('update-url'),
        '',
        $menu.data('issue-id'),
        $(this).data('id'),
      ).then(reload);
    }

    $list.find('.selected').html('');
    $list.find('.no-select').removeClass('hide');
    $(input_id).val('');
  });
}

function initFilterBranchTagDropdown(selector) {
  $(selector).each(function () {
    const $dropdown = $(this);
    const $data = $dropdown.find('.data');
    const data = {
      items: [],
      mode: $data.data('mode'),
      searchTerm: '',
      noResults: '',
      canCreateBranch: false,
      menuVisible: false,
      active: 0
    };
    $data.find('.item').each(function () {
      data.items.push({
        name: $(this).text(),
        url: $(this).data('url'),
        branch: $(this).hasClass('branch'),
        tag: $(this).hasClass('tag'),
        selected: $(this).hasClass('selected')
      });
    });
    $data.remove();
    new Vue({
      el: this,
      delimiters: ['${', '}'],
      data,
      computed: {
        filteredItems() {
          const items = this.items.filter((item) => {
            return ((this.mode === 'branches' && item.branch) || (this.mode === 'tags' && item.tag)) &&
              (!this.searchTerm || item.name.toLowerCase().includes(this.searchTerm.toLowerCase()));
          });

          // no idea how to fix this so linting rule is disabled instead
          this.active = (items.length === 0 && this.showCreateNewBranch ? 0 : -1); // eslint-disable-line vue/no-side-effects-in-computed-properties
          return items;
        },
        showNoResults() {
          return this.filteredItems.length === 0 && !this.showCreateNewBranch;
        },
        showCreateNewBranch() {
          if (!this.canCreateBranch || !this.searchTerm || this.mode === 'tags') {
            return false;
          }

          return this.items.filter((item) => item.name.toLowerCase() === this.searchTerm.toLowerCase()).length === 0;
        }
      },

      watch: {
        menuVisible(visible) {
          if (visible) {
            this.focusSearchField();
          }
        }
      },

      beforeMount() {
        this.noResults = this.$el.getAttribute('data-no-results');
        this.canCreateBranch = this.$el.getAttribute('data-can-create-branch') === 'true';

        document.body.addEventListener('click', (event) => {
          if (this.$el.contains(event.target)) return;
          if (this.menuVisible) {
            Vue.set(this, 'menuVisible', false);
          }
        });
      },

      methods: {
        selectItem(item) {
          const prev = this.getSelected();
          if (prev !== null) {
            prev.selected = false;
          }
          item.selected = true;
          window.location.href = item.url;
        },
        createNewBranch() {
          if (!this.showCreateNewBranch) return;
          $(this.$refs.newBranchForm).trigger('submit');
        },
        focusSearchField() {
          Vue.nextTick(() => {
            this.$refs.searchField.focus();
          });
        },
        getSelected() {
          for (let i = 0, j = this.items.length; i < j; ++i) {
            if (this.items[i].selected) return this.items[i];
          }
          return null;
        },
        getSelectedIndexInFiltered() {
          for (let i = 0, j = this.filteredItems.length; i < j; ++i) {
            if (this.filteredItems[i].selected) return i;
          }
          return -1;
        },
        scrollToActive() {
          let el = this.$refs[`listItem${this.active}`];
          if (!el || !el.length) return;
          if (Array.isArray(el)) {
            el = el[0];
          }

          const cont = this.$refs.scrollContainer;
          if (el.offsetTop < cont.scrollTop) {
            cont.scrollTop = el.offsetTop;
          } else if (el.offsetTop + el.clientHeight > cont.scrollTop + cont.clientHeight) {
            cont.scrollTop = el.offsetTop + el.clientHeight - cont.clientHeight;
          }
        },
        keydown(event) {
          if (event.keyCode === 40) { // arrow down
            event.preventDefault();

            if (this.active === -1) {
              this.active = this.getSelectedIndexInFiltered();
            }

            if (this.active + (this.showCreateNewBranch ? 0 : 1) >= this.filteredItems.length) {
              return;
            }
            this.active++;
            this.scrollToActive();
          } else if (event.keyCode === 38) { // arrow up
            event.preventDefault();

            if (this.active === -1) {
              this.active = this.getSelectedIndexInFiltered();
            }

            if (this.active <= 0) {
              return;
            }
            this.active--;
            this.scrollToActive();
          } else if (event.keyCode === 13) { // enter
            event.preventDefault();

            if (this.active >= this.filteredItems.length) {
              this.createNewBranch();
            } else if (this.active >= 0) {
              this.selectItem(this.filteredItems[this.active]);
            }
          } else if (event.keyCode === 27) { // escape
            event.preventDefault();
            this.menuVisible = false;
          }
        }
      }
    });
  });
}
function initBranchSelector() {
  const $selectBranch = $('.ui.select-branch');
  const $branchMenu = $selectBranch.find('.reference-list-menu');
  $branchMenu.find('.item:not(.no-select)').click(function () {
    const selectedValue = $(this).data('id');
    const editMode = $('#editing_mode').val();
    $($(this).data('id-selector')).val(selectedValue);

    if (editMode === 'true') {
      const form = $('#update_issueref_form');

      $.post(form.attr('action'), {
        _csrf: csrf,
        ref: selectedValue
      },
      () => {
        window.location.reload();
      });
    } else if (editMode === '') {
      $selectBranch.find('.ui .branch-name').text(selectedValue);
    }
  });
  $selectBranch.find('.reference.column').on('click', function () {
    $selectBranch.find('.scrolling.reference-list-menu').css('display', 'none');
    $selectBranch.find('.reference .text').removeClass('black');
    $($(this).data('target')).css('display', 'block');
    $(this).find('.text').addClass('black');
    return false;
  });
}

// Listsubmit
function initListSubmits(selector, outerSelector) {
  const $list = $(`.ui.${outerSelector}.list`);
  const $noSelect = $list.find('.no-select');
  const $listMenu = $(`.${selector} .menu`);
  let hasUpdateAction = $listMenu.data('action') === 'update';
  const items = {};

  $(`.${selector}`).dropdown('setting', 'onHide', () => {
    hasUpdateAction = $listMenu.data('action') === 'update'; // Update the var
    if (hasUpdateAction) {
      const promises = [];
      Object.keys(items).forEach((elementId) => {
        const item = items[elementId];
        const promise = updateIssuesMeta(
          item['update-url'],
          item.action,
          item['issue-id'],
          elementId,
        );
        promises.push(promise);
      });
      Promise.all(promises).then(reloadIssuesActions);
    }
  });

  $listMenu.find('.item:not(.no-select)').on('click', function (e) {
    e.preventDefault();
    if ($(this).hasClass('ban-change')) {
      return false;
    }

    hasUpdateAction = $listMenu.data('action') === 'update'; // Update the var
    if ($(this).hasClass('checked')) {
      $(this).removeClass('checked');
      $(this).find('.octicon-check').addClass('invisible');
      if (hasUpdateAction) {
        if (!($(this).data('id') in items)) {
          items[$(this).data('id')] = {
            'update-url': $listMenu.data('update-url'),
            action: 'detach',
            'issue-id': $listMenu.data('issue-id'),
          };
        } else {
          delete items[$(this).data('id')];
        }
      }
    } else {
      $(this).addClass('checked');
      $(this).find('.octicon-check').removeClass('invisible');
      if (hasUpdateAction) {
        if (!($(this).data('id') in items)) {
          items[$(this).data('id')] = {
            'update-url': $listMenu.data('update-url'),
            action: 'attach',
            'issue-id': $listMenu.data('issue-id'),
          };
        } else {
          delete items[$(this).data('id')];
        }
      }
    }

    // TODO: Which thing should be done for choosing review requests
    // to make choosed items be shown on time here?
    if (selector === 'select-reviewers-modify' || selector === 'select-assignees-modify') {
      return false;
    }

    const listIds = [];
    $(this).parent().find('.item').each(function () {
      if ($(this).hasClass('checked')) {
        listIds.push($(this).data('id'));
        $($(this).data('id-selector')).removeClass('hide');
      } else {
        $($(this).data('id-selector')).addClass('hide');
      }
    });
    if (listIds.length === 0) {
      $noSelect.removeClass('hide');
    } else {
      $noSelect.addClass('hide');
    }
    $($(this).parent().data('id')).val(listIds.join(','));
    return false;
  });
  $listMenu.find('.no-select.item').on('click', function (e) {
    e.preventDefault();
    if (hasUpdateAction) {
      updateIssuesMeta(
        $listMenu.data('update-url'),
        'clear',
        $listMenu.data('issue-id'),
        '',
      ).then(reload);
    }

    $(this).parent().find('.item').each(function () {
      $(this).removeClass('checked');
      $(this).find('.octicon').addClass('invisible');
    });

    if (selector === 'select-reviewers-modify' || selector === 'select-assignees-modify') {
      return false;
    }

    $list.find('.item').each(function () {
      $(this).addClass('hide');
    });
    $noSelect.removeClass('hide');
    $($(this).parent().data('id')).val('');
  });
}

