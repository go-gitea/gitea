import $ from 'jquery';
import {createCommentEasyMDE, getAttachedEasyMDE} from './comp/EasyMDE.js';
import {initCompMarkupContentPreviewTab} from './comp/MarkupContentPreview.js';
import {initEasyMDEImagePaste} from './comp/ImagePaste.js';
import {
  initRepoIssueBranchSelect, initRepoIssueCodeCommentCancel,
  initRepoIssueCommentDelete,
  initRepoIssueComments, initRepoIssueDependencyDelete,
  initRepoIssueReferenceIssue, initRepoIssueStatusButton,
  initRepoIssueTitleEdit,
  initRepoIssueWipToggle, initRepoPullRequestUpdate,
  updateIssuesMeta,
} from './repo-issue.js';
import {initUnicodeEscapeButton} from './repo-unicode-escape.js';
import {svg} from '../svg.js';
import {htmlEscape} from 'escape-goat';
import {initRepoBranchTagDropdown} from '../components/RepoBranchTagDropdown.js';
import {
  initRepoCloneLink,
  initRepoCommonBranchOrTagDropdown,
  initRepoCommonFilterSearchDropdown,
  initRepoCommonLanguageStats,
} from './repo-common.js';
import {initCompLabelEdit} from './comp/LabelEdit.js';
import {initRepoDiffConversationNav} from './repo-diff.js';
import attachTribute from './tribute.js';
import createDropzone from './dropzone.js';
import {initCommentContent, initMarkupContent} from '../markup/content.js';
import {initCompReactionSelector} from './comp/ReactionSelector.js';
import {initRepoSettingBranches} from './repo-settings.js';
import initRepoPullRequestMergeForm from './repo-issue-pr-form.js';

const {csrfToken} = window.config;

export function initRepoCommentForm() {
  const $commentForm = $('.comment.form');
  if ($commentForm.length === 0) {
    return;
  }

  function initBranchSelector() {
    const $selectBranch = $('.ui.select-branch');
    const $branchMenu = $selectBranch.find('.reference-list-menu');
    const $isNewIssue = $branchMenu.hasClass('new-issue');
    $branchMenu.find('.item:not(.no-select)').click(function () {
      const selectedValue = $(this).data('id');
      const editMode = $('#editing_mode').val();
      $($(this).data('id-selector')).val(selectedValue);
      if ($isNewIssue) {
        $selectBranch.find('.ui .branch-name').text($(this).data('name'));
        return;
      }

      if (editMode === 'true') {
        const form = $('#update_issueref_form');
        $.post(form.attr('action'), {_csrf: csrfToken, ref: selectedValue}, () => window.location.reload());
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

  (async () => {
    for (const textarea of $commentForm.find('textarea:not(.review-textarea, .no-easymde)')) {
      // Don't initialize EasyMDE for the dormant #edit-content-form
      if (textarea.closest('#edit-content-form')) {
        continue;
      }
      const easyMDE = await createCommentEasyMDE(textarea);
      initEasyMDEImagePaste(easyMDE, $commentForm.find('.dropzone'));
    }
  })();

  initBranchSelector();
  initCompMarkupContentPreviewTab($commentForm);

  // List submits
  function initListSubmits(selector, outerSelector) {
    const $list = $(`.ui.${outerSelector}.list`);
    const $noSelect = $list.find('.no-select');
    const $listMenu = $(`.${selector} .menu`);
    let hasUpdateAction = $listMenu.data('action') === 'update';
    const items = {};

    $(`.${selector}`).dropdown('setting', 'onHide', () => {
      hasUpdateAction = $listMenu.data('action') === 'update'; // Update the var
      if (hasUpdateAction) {
        // TODO: Add batch functionality and make this 1 network request.
        (async function() {
          for (const [elementId, item] of Object.entries(items)) {
            await updateIssuesMeta(
              item['update-url'],
              item.action,
              item['issue-id'],
              elementId,
            );
          }
          window.location.reload();
        })();
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
      // to make chosen items be shown on time here?
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
        ).then(() => window.location.reload());
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

  // Init labels and assignees
  initListSubmits('select-label', 'labels');
  initListSubmits('select-assignees', 'assignees');
  initListSubmits('select-assignees-modify', 'assignees');
  initListSubmits('select-reviewers-modify', 'assignees');

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
        ).then(() => window.location.reload());
      }

      let icon = '';
      if (input_id === '#milestone_id') {
        icon = svg('octicon-milestone', 18, 'mr-3');
      } else if (input_id === '#project_id') {
        icon = svg('octicon-project', 18, 'mr-3');
      } else if (input_id === '#assignee_id') {
        icon = `<img class="ui avatar image mr-3" src=${$(this).data('avatar')}>`;
      }

      $list.find('.selected').html(`
        <a class="item muted sidebar-item-link" href=${$(this).data('href')}>
          ${icon}
          ${htmlEscape($(this).text())}
        </a>
      `);

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
        ).then(() => window.location.reload());
      }

      $list.find('.selected').html('');
      $list.find('.no-select').removeClass('hide');
      $(input_id).val('');
    });
  }

  // Milestone, Assignee, Project
  selectItem('.select-project', '#project_id');
  selectItem('.select-milestone', '#milestone_id');
  selectItem('.select-assignee', '#assignee_id');
}


async function onEditContent(event) {
  event.preventDefault();

  $(this).closest('.dropdown').find('.menu').toggle('visible');
  const $segment = $(this).closest('.header').next();
  const $editContentZone = $segment.find('.edit-content-zone');
  const $renderContent = $segment.find('.render-content');
  const $rawContent = $segment.find('.raw-content');
  let $textarea;
  let easyMDE;

  // Setup new form
  if ($editContentZone.html().length === 0) {
    $editContentZone.html($('#edit-content-form').html());
    $textarea = $editContentZone.find('textarea');
    await attachTribute($textarea.get(), {mentions: true, emoji: true});

    let dz;
    const $dropzone = $editContentZone.find('.dropzone');
    if ($dropzone.length === 1) {
      $dropzone.data('saved', false);

      const fileUuidDict = {};
      dz = await createDropzone($dropzone[0], {
        url: $dropzone.data('upload-url'),
        headers: {'X-Csrf-Token': csrfToken},
        maxFiles: $dropzone.data('max-file'),
        maxFilesize: $dropzone.data('max-size'),
        acceptedFiles: (['*/*', ''].includes($dropzone.data('accepts'))) ? null : $dropzone.data('accepts'),
        addRemoveLinks: true,
        dictDefaultMessage: $dropzone.data('default-message'),
        dictInvalidFileType: $dropzone.data('invalid-input-type'),
        dictFileTooBig: $dropzone.data('file-too-big'),
        dictRemoveFile: $dropzone.data('remove-file'),
        timeout: 0,
        thumbnailMethod: 'contain',
        thumbnailWidth: 480,
        thumbnailHeight: 480,
        init() {
          this.on('success', (file, data) => {
            file.uuid = data.uuid;
            fileUuidDict[file.uuid] = {submitted: false};
            const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
            $dropzone.find('.files').append(input);
          });
          this.on('removedfile', (file) => {
            $(`#${file.uuid}`).remove();
            if ($dropzone.data('remove-url') && !fileUuidDict[file.uuid].submitted) {
              $.post($dropzone.data('remove-url'), {
                file: file.uuid,
                _csrf: csrfToken,
              });
            }
          });
          this.on('submit', () => {
            $.each(fileUuidDict, (fileUuid) => {
              fileUuidDict[fileUuid].submitted = true;
            });
          });
          this.on('reload', () => {
            $.getJSON($editContentZone.data('attachment-url'), (data) => {
              dz.removeAllFiles(true);
              $dropzone.find('.files').empty();
              $.each(data, function () {
                const imgSrc = `${$dropzone.data('link-url')}/${this.uuid}`;
                dz.emit('addedfile', this);
                dz.emit('thumbnail', this, imgSrc);
                dz.emit('complete', this);
                dz.files.push(this);
                fileUuidDict[this.uuid] = {submitted: true};
                $dropzone.find(`img[src='${imgSrc}']`).css('max-width', '100%');
                const input = $(`<input id="${this.uuid}" name="files" type="hidden">`).val(this.uuid);
                $dropzone.find('.files').append(input);
              });
            });
          });
        },
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
    easyMDE = await createCommentEasyMDE($textarea);

    initCompMarkupContentPreviewTab($editContentForm);
    initEasyMDEImagePaste(easyMDE, $dropzone);

    const $saveButton = $editContentZone.find('.save.button');
    $textarea.on('ce-quick-submit', () => {
      $saveButton.trigger('click');
    });

    $editContentZone.find('.cancel.button').on('click', () => {
      $renderContent.show();
      $editContentZone.hide();
      if (dz) {
        dz.emit('reload');
      }
    });

    $saveButton.on('click', () => {
      $renderContent.show();
      $editContentZone.hide();
      const $attachments = $dropzone.find('.files').find('[name=files]').map(function () {
        return $(this).val();
      }).get();
      $.post($editContentZone.data('update-url'), {
        _csrf: csrfToken,
        content: $textarea.val(),
        context: $editContentZone.data('context'),
        files: $attachments,
      }, (data) => {
        if (data.length === 0 || data.content.length === 0) {
          $renderContent.html($('#no-content').html());
          $rawContent.text('');
        } else {
          $renderContent.html(data.content);
          $rawContent.text($textarea.val());
        }
        const $content = $segment;
        if (!$content.find('.dropzone-attachments').length) {
          if (data.attachments !== '') {
            $content.append(`<div class="dropzone-attachments"></div>`);
            $content.find('.dropzone-attachments').replaceWith(data.attachments);
          }
        } else if (data.attachments === '') {
          $content.find('.dropzone-attachments').remove();
        } else {
          $content.find('.dropzone-attachments').replaceWith(data.attachments);
        }
        if (dz) {
          dz.emit('submit');
          dz.emit('reload');
        }
        initMarkupContent();
        initCommentContent();
      });
    });
  } else { // use existing form
    $textarea = $segment.find('textarea');
    easyMDE = getAttachedEasyMDE($textarea);
  }

  // Show write/preview tab and copy raw content as needed
  $editContentZone.show();
  $renderContent.hide();
  if ($textarea.val().length === 0) {
    $textarea.val($rawContent.text());
    easyMDE.value($rawContent.text());
  }
  requestAnimationFrame(() => {
    $textarea.focus();
    easyMDE.codemirror.focus();
  });
}

export function initRepository() {
  if ($('.repository').length === 0) {
    return;
  }


  // File list and commits
  if ($('.repository.file.list').length > 0 || $('.branch-dropdown').length > 0 ||
    $('.repository.commits').length > 0 || $('.repository.release').length > 0) {
    initRepoBranchTagDropdown('.choose.reference .dropdown');
  }

  // Wiki
  if ($('.repository.wiki.view').length > 0) {
    initRepoCommonFilterSearchDropdown('.choose.page .dropdown');
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
    const $trackerIssueStyleRadios = $('.js-tracker-issue-style');
    $trackerIssueStyleRadios.on('change input', () => {
      const checkedVal = $trackerIssueStyleRadios.filter(':checked').val();
      $('#tracker-issue-style-regex-box').toggleClass('disabled', checkedVal !== 'regexp');
    });
  }

  // Labels
  initCompLabelEdit('.repository.labels');

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

  // Compare or pull request
  const $repoDiff = $('.repository.diff');
  if ($repoDiff.length) {
    initRepoCommonBranchOrTagDropdown('.choose.branch .dropdown');
    initRepoCommonFilterSearchDropdown('.choose.branch .dropdown');
  }

  initRepoCloneLink();
  initRepoCommonLanguageStats();
  initRepoSettingBranches();

  // Issues
  if ($('.repository.view.issue').length > 0) {
    initRepoIssueCommentEdit();

    initRepoIssueBranchSelect();
    initRepoIssueTitleEdit();
    initRepoIssueWipToggle();
    initRepoIssueComments();

    initRepoDiffConversationNav();
    initRepoIssueReferenceIssue();


    initRepoIssueCommentDelete();
    initRepoIssueDependencyDelete();
    initRepoIssueCodeCommentCancel();
    initRepoIssueStatusButton();
    initRepoPullRequestUpdate();
    initCompReactionSelector();

    initRepoPullRequestMergeForm();
  }

  // Pull request
  const $repoComparePull = $('.repository.compare.pull');
  if ($repoComparePull.length > 0) {
    // show pull request form
    $repoComparePull.find('button.show-form').on('click', function (e) {
      e.preventDefault();
      $(this).parent().hide();

      const $form = $repoComparePull.find('.pullrequest-form');
      $form.show();
      $form.find('textarea.edit_area').each(function() {
        const easyMDE = getAttachedEasyMDE($(this));
        if (easyMDE) {
          easyMDE.codemirror.refresh();
        }
      });
    });
  }

  initUnicodeEscapeButton();
}

function initRepoIssueCommentEdit() {
  // Issue/PR Context Menus
  $('.comment-header-right .context-dropdown').dropdown({action: 'hide'});

  // Edit issue or comment content
  $(document).on('click', '.edit-content', onEditContent);

  // Quote reply
  $(document).on('click', '.quote-reply', function (event) {
    $(this).closest('.dropdown').find('.menu').toggle('visible');
    const target = $(this).data('target');
    const quote = $(`#comment-${target}`).text().replace(/\n/g, '\n> ');
    const content = `> ${quote}\n\n`;
    let easyMDE;
    if ($(this).hasClass('quote-reply-diff')) {
      const $parent = $(this).closest('.comment-code-cloud');
      $parent.find('button.comment-form-reply').trigger('click');
      easyMDE = getAttachedEasyMDE($parent.find('[name="content"]'));
    } else {
      // for normal issue/comment page
      easyMDE = getAttachedEasyMDE($('#comment-form .edit_area'));
    }
    if (easyMDE) {
      if (easyMDE.value() !== '') {
        easyMDE.value(`${easyMDE.value()}\n\n${content}`);
      } else {
        easyMDE.value(`${content}`);
      }
      requestAnimationFrame(() => {
        easyMDE.codemirror.focus();
        easyMDE.codemirror.setCursor(easyMDE.codemirror.lineCount(), 0);
      });
    }
    event.preventDefault();
  });
}
