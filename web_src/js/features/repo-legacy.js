import $ from 'jquery';
import {
  initRepoIssueBranchSelect, initRepoIssueCodeCommentCancel, initRepoIssueCommentDelete,
  initRepoIssueComments, initRepoIssueDependencyDelete, initRepoIssueReferenceIssue,
  initRepoIssueTitleEdit, initRepoIssueWipToggle,
  initRepoPullRequestUpdate, updateIssuesMeta, handleReply, initIssueTemplateCommentEditors, initSingleCommentEditor,
} from './repo-issue.js';
import {initUnicodeEscapeButton} from './repo-unicode-escape.js';
import {svg} from '../svg.js';
import {htmlEscape} from 'escape-goat';
import {initRepoBranchTagSelector} from '../components/RepoBranchTagSelector.vue';
import {
  initRepoCloneLink, initRepoCommonBranchOrTagDropdown, initRepoCommonFilterSearchDropdown,
} from './repo-common.js';
import {initCitationFileCopyContent} from './citation.js';
import {initCompLabelEdit} from './comp/LabelEdit.js';
import {initRepoDiffConversationNav} from './repo-diff.js';
import {createDropzone} from './dropzone.js';
import {initCommentContent, initMarkupContent} from '../markup/content.js';
import {initCompReactionSelector} from './comp/ReactionSelector.js';
import {initRepoSettingBranches} from './repo-settings.js';
import {initRepoPullRequestMergeForm} from './repo-issue-pr-form.js';
import {initRepoPullRequestCommitStatus} from './repo-issue-pr-status.js';
import {hideElem, showElem} from '../utils/dom.js';
import {getComboMarkdownEditor, initComboMarkdownEditor} from './comp/ComboMarkdownEditor.js';
import {attachRefIssueContextPopup} from './contextpopup.js';
import {POST, GET} from '../modules/fetch.js';

const {csrfToken} = window.config;

// if there are draft comments, confirm before reloading, to avoid losing comments
function reloadConfirmDraftComment() {
  const commentTextareas = [
    document.querySelector('.edit-content-zone:not(.tw-hidden) textarea'),
    document.querySelector('#comment-form textarea'),
  ];
  for (const textarea of commentTextareas) {
    // Most users won't feel too sad if they lose a comment with 10 chars, they can re-type these in seconds.
    // But if they have typed more (like 50) chars and the comment is lost, they will be very unhappy.
    if (textarea && textarea.value.trim().length > 10) {
      textarea.parentElement.scrollIntoView();
      if (!window.confirm('Page will be reloaded, but there are draft comments. Continuing to reload will discard the comments. Continue?')) {
        return;
      }
      break;
    }
  }
  window.location.reload();
}

export function initRepoCommentForm() {
  const $commentForm = $('.comment.form');
  if (!$commentForm.length) return;

  if ($commentForm.find('.field.combo-editor-dropzone').length) {
    // at the moment, if a form has multiple combo-markdown-editors, it must be an issue template form
    initIssueTemplateCommentEditors($commentForm);
  } else if ($commentForm.find('.combo-markdown-editor').length) {
    // it's quite unclear about the "comment form" elements, sometimes it's for issue comment, sometimes it's for file editor/uploader message
    initSingleCommentEditor($commentForm);
  }

  function initBranchSelector() {
    const $selectBranch = $('.ui.select-branch');
    const $branchMenu = $selectBranch.find('.reference-list-menu');
    const $isNewIssue = $branchMenu.hasClass('new-issue');
    $branchMenu.find('.item:not(.no-select)').on('click', async function () {
      const selectedValue = $(this).data('id');
      const editMode = $('#editing_mode').val();
      $($(this).data('id-selector')).val(selectedValue);
      if ($isNewIssue) {
        $selectBranch.find('.ui .branch-name').text($(this).data('name'));
        return;
      }

      if (editMode === 'true') {
        const form = document.getElementById('update_issueref_form');
        const params = new URLSearchParams();
        params.append('ref', selectedValue);
        try {
          await POST(form.getAttribute('action'), {data: params});
          window.location.reload();
        } catch (error) {
          console.error(error);
        }
      } else if (editMode === '') {
        $selectBranch.find('.ui .branch-name').text(selectedValue);
      }
    });
    $selectBranch.find('.reference.column').on('click', function () {
      hideElem($selectBranch.find('.scrolling.reference-list-menu'));
      $selectBranch.find('.reference .text').removeClass('black');
      showElem($($(this).data('target')));
      $(this).find('.text').addClass('black');
      return false;
    });
  }

  initBranchSelector();

  // List submits
  function initListSubmits(selector, outerSelector) {
    const $list = $(`.ui.${outerSelector}.list`);
    const $noSelect = $list.find('.no-select');
    const $listMenu = $(`.${selector} .menu`);
    let hasUpdateAction = $listMenu.data('action') === 'update';
    const items = {};

    $(`.${selector}`).dropdown({
      'action': 'nothing', // do not hide the menu if user presses Enter
      fullTextSearch: 'exact',
      async onHide() {
        hasUpdateAction = $listMenu.data('action') === 'update'; // Update the var
        if (hasUpdateAction) {
          // TODO: Add batch functionality and make this 1 network request.
          const itemEntries = Object.entries(items);
          for (const [elementId, item] of itemEntries) {
            await updateIssuesMeta(
              item['update-url'],
              item.action,
              item['issue-id'],
              elementId,
            );
          }
          if (itemEntries.length) {
            reloadConfirmDraftComment();
          }
        }
      },
    });

    $listMenu.find('.item:not(.no-select)').on('click', function (e) {
      e.preventDefault();
      if ($(this).hasClass('ban-change')) {
        return false;
      }

      hasUpdateAction = $listMenu.data('action') === 'update'; // Update the var

      const clickedItem = this; // eslint-disable-line unicorn/no-this-assignment
      const scope = this.getAttribute('data-scope');

      $(this).parent().find('.item').each(function () {
        if (scope) {
          // Enable only clicked item for scoped labels
          if (this.getAttribute('data-scope') !== scope) {
            return true;
          }
          if (this !== clickedItem && !$(this).hasClass('checked')) {
            return true;
          }
        } else if (this !== clickedItem) {
          // Toggle for other labels
          return true;
        }

        if ($(this).hasClass('checked')) {
          $(this).removeClass('checked');
          $(this).find('.octicon-check').addClass('tw-invisible');
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
          $(this).find('.octicon-check').removeClass('tw-invisible');
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
      });

      // TODO: Which thing should be done for choosing review requests
      // to make chosen items be shown on time here?
      if (selector === 'select-reviewers-modify' || selector === 'select-assignees-modify') {
        return false;
      }

      const listIds = [];
      $(this).parent().find('.item').each(function () {
        if ($(this).hasClass('checked')) {
          listIds.push($(this).data('id'));
          $($(this).data('id-selector')).removeClass('tw-hidden');
        } else {
          $($(this).data('id-selector')).addClass('tw-hidden');
        }
      });
      if (!listIds.length) {
        $noSelect.removeClass('tw-hidden');
      } else {
        $noSelect.addClass('tw-hidden');
      }
      $($(this).parent().data('id')).val(listIds.join(','));
      return false;
    });
    $listMenu.find('.no-select.item').on('click', function (e) {
      e.preventDefault();
      if (hasUpdateAction) {
        (async () => {
          await updateIssuesMeta(
            $listMenu.data('update-url'),
            'clear',
            $listMenu.data('issue-id'),
            '',
          );
          reloadConfirmDraftComment();
        })();
      }

      $(this).parent().find('.item').each(function () {
        $(this).removeClass('checked');
        $(this).find('.octicon-check').addClass('tw-invisible');
      });

      if (selector === 'select-reviewers-modify' || selector === 'select-assignees-modify') {
        return false;
      }

      $list.find('.item').each(function () {
        $(this).addClass('tw-hidden');
      });
      $noSelect.removeClass('tw-hidden');
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
        (async () => {
          await updateIssuesMeta(
            $menu.data('update-url'),
            '',
            $menu.data('issue-id'),
            $(this).data('id'),
          );
          reloadConfirmDraftComment();
        })();
      }

      let icon = '';
      if (input_id === '#milestone_id') {
        icon = svg('octicon-milestone', 18, 'tw-mr-2');
      } else if (input_id === '#project_id') {
        icon = svg('octicon-project', 18, 'tw-mr-2');
      } else if (input_id === '#assignee_id') {
        icon = `<img class="ui avatar image tw-mr-2" alt="avatar" src=${$(this).data('avatar')}>`;
      }

      $list.find('.selected').html(`
        <a class="item muted sidebar-item-link" href=${$(this).data('href')}>
          ${icon}
          ${htmlEscape($(this).text())}
        </a>
      `);

      $(`.ui${select_id}.list .no-select`).addClass('tw-hidden');
      $(input_id).val($(this).data('id'));
    });
    $menu.find('.no-select.item').on('click', function () {
      $(this).parent().find('.item:not(.no-select)').each(function () {
        $(this).removeClass('selected active');
      });

      if (hasUpdateAction) {
        (async () => {
          await updateIssuesMeta(
            $menu.data('update-url'),
            '',
            $menu.data('issue-id'),
            $(this).data('id'),
          );
          reloadConfirmDraftComment();
        })();
      }

      $list.find('.selected').html('');
      $list.find('.no-select').removeClass('tw-hidden');
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

  const segment = this.closest('.header').nextElementSibling;
  const editContentZone = segment.querySelector('.edit-content-zone');
  const renderContent = segment.querySelector('.render-content');
  const rawContent = segment.querySelector('.raw-content');

  let comboMarkdownEditor;

  /**
   * @param {HTMLElement} dropzone
   */
  const setupDropzone = async (dropzone) => {
    if (!dropzone) return null;

    let disableRemovedfileEvent = false; // when resetting the dropzone (removeAllFiles), disable the "removedfile" event
    let fileUuidDict = {}; // to record: if a comment has been saved, then the uploaded files won't be deleted from server when clicking the Remove in the dropzone
    const dz = await createDropzone(dropzone, {
      url: dropzone.getAttribute('data-upload-url'),
      headers: {'X-Csrf-Token': csrfToken},
      maxFiles: dropzone.getAttribute('data-max-file'),
      maxFilesize: dropzone.getAttribute('data-max-size'),
      acceptedFiles: ['*/*', ''].includes(dropzone.getAttribute('data-accepts')) ? null : dropzone.getAttribute('data-accepts'),
      addRemoveLinks: true,
      dictDefaultMessage: dropzone.getAttribute('data-default-message'),
      dictInvalidFileType: dropzone.getAttribute('data-invalid-input-type'),
      dictFileTooBig: dropzone.getAttribute('data-file-too-big'),
      dictRemoveFile: dropzone.getAttribute('data-remove-file'),
      timeout: 0,
      thumbnailMethod: 'contain',
      thumbnailWidth: 480,
      thumbnailHeight: 480,
      init() {
        this.on('success', (file, data) => {
          file.uuid = data.uuid;
          fileUuidDict[file.uuid] = {submitted: false};
          const input = document.createElement('input');
          input.id = data.uuid;
          input.name = 'files';
          input.type = 'hidden';
          input.value = data.uuid;
          dropzone.querySelector('.files').insertAdjacentHTML('beforeend', input.outerHTML);
        });
        this.on('removedfile', async (file) => {
          if (disableRemovedfileEvent) return;
          document.getElementById(file.uuid)?.remove();
          if (dropzone.getAttribute('data-remove-url') && !fileUuidDict[file.uuid].submitted) {
            try {
              await POST(dropzone.getAttribute('data-remove-url'), {data: new URLSearchParams({file: file.uuid})});
            } catch (error) {
              console.error(error);
            }
          }
        });
        this.on('submit', () => {
          for (const fileUuid of Object.keys(fileUuidDict)) {
            fileUuidDict[fileUuid].submitted = true;
          }
        });
        this.on('reload', async () => {
          try {
            const response = await GET(editContentZone.getAttribute('data-attachment-url'));
            const data = await response.json();
            // do not trigger the "removedfile" event, otherwise the attachments would be deleted from server
            disableRemovedfileEvent = true;
            dz.removeAllFiles(true);
            dropzone.querySelector('.files').innerHTML = '';
            fileUuidDict = {};
            disableRemovedfileEvent = false;

            for (const attachment of data) {
              const imgSrc = `${dropzone.getAttribute('data-link-url')}/${attachment.uuid}`;
              dz.emit('addedfile', attachment);
              dz.emit('thumbnail', attachment, imgSrc);
              dz.emit('complete', attachment);
              dz.files.push(attachment);
              fileUuidDict[attachment.uuid] = {submitted: true};
              dropzone.querySelector(`img[src='${imgSrc}']`).style.maxWidth = '100%';
              const input = document.createElement('input');
              input.id = attachment.uuid;
              input.name = 'files';
              input.type = 'hidden';
              input.value = attachment.uuid;
              dropzone.querySelector('.files').insertAdjacentHTML('beforeend', input.outerHTML);
            }
          } catch (error) {
            console.error(error);
          }
        });
      },
    });
    dz.emit('reload');
    return dz;
  };

  const cancelAndReset = (dz) => {
    showElem(renderContent);
    hideElem(editContentZone);
    if (dz) {
      dz.emit('reload');
    }
  };

  const saveAndRefresh = async (dz) => {
    showElem(renderContent);
    hideElem(editContentZone);

    try {
      const params = new URLSearchParams({
        content: comboMarkdownEditor.value(),
        context: editContentZone.getAttribute('data-context'),
      });
      for (const file of dz.files) params.append('files[]', file.uuid);

      const response = await POST(editContentZone.getAttribute('data-update-url'), {data: params});
      const data = await response.json();
      if (!data.content) {
        renderContent.innerHTML = document.getElementById('no-content').innerHTML;
        rawContent.textContent = '';
      } else {
        renderContent.innerHTML = data.content;
        rawContent.textContent = comboMarkdownEditor.value();
        const refIssues = renderContent.querySelectorAll('p .ref-issue');
        attachRefIssueContextPopup(refIssues);
      }
      const content = segment;
      if (!content.querySelector('.dropzone-attachments')) {
        if (data.attachments !== '') {
          content.insertAdjacentHTML('beforeend', data.attachments);
        }
      } else if (data.attachments === '') {
        content.querySelector('.dropzone-attachments').remove();
      } else {
        content.querySelector('.dropzone-attachments').outerHTML = data.attachments;
      }
      if (dz) {
        dz.emit('submit');
        dz.emit('reload');
      }
      initMarkupContent();
      initCommentContent();
    } catch (error) {
      console.error(error);
    }
  };

  if (!editContentZone.innerHTML) {
    editContentZone.innerHTML = document.getElementById('issue-comment-editor-template').innerHTML;
    comboMarkdownEditor = await initComboMarkdownEditor(editContentZone.querySelector('.combo-markdown-editor'));

    const dropzone = editContentZone.querySelector('.dropzone');
    const dz = await setupDropzone(dropzone);
    editContentZone.querySelector('.cancel.button').addEventListener('click', (e) => {
      e.preventDefault();
      cancelAndReset(dz);
    });
    editContentZone.querySelector('.save.button').addEventListener('click', (e) => {
      e.preventDefault();
      saveAndRefresh(dz);
    });
  } else {
    comboMarkdownEditor = getComboMarkdownEditor(editContentZone.querySelector('.combo-markdown-editor'));
  }

  // Show write/preview tab and copy raw content as needed
  showElem(editContentZone);
  hideElem(renderContent);
  if (!comboMarkdownEditor.value()) {
    comboMarkdownEditor.value(rawContent.textContent);
  }
  comboMarkdownEditor.focus();
}

export function initRepository() {
  if (!$('.page-content.repository').length) return;

  initRepoBranchTagSelector('.js-branch-tag-selector');

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
        if ($(this).data('context') !== undefined) $($(this).data('context')).removeClass('disabled');
      } else if (this.value === 'true') {
        $($(this).data('target')).removeClass('disabled');
        if ($(this).data('context') !== undefined) $($(this).data('context')).addClass('disabled');
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
        document.querySelector('input[name="auto_init"]').checked = true;
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
  initCitationFileCopyContent();
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
    initRepoPullRequestUpdate();
    initCompReactionSelector($(document));

    initRepoPullRequestMergeForm();
    initRepoPullRequestCommitStatus();
  }

  // Pull request
  const $repoComparePull = $('.repository.compare.pull');
  if ($repoComparePull.length > 0) {
    // show pull request form
    $repoComparePull.find('button.show-form').on('click', function (e) {
      e.preventDefault();
      hideElem($(this).parent());

      const $form = $repoComparePull.find('.pullrequest-form');
      showElem($form);
    });
  }

  initUnicodeEscapeButton();
}

function initRepoIssueCommentEdit() {
  // Edit issue or comment content
  $(document).on('click', '.edit-content', onEditContent);

  // Quote reply
  $(document).on('click', '.quote-reply', async function (event) {
    event.preventDefault();
    const target = $(this).data('target');
    const quote = $(`#${target}`).text().replace(/\n/g, '\n> ');
    const content = `> ${quote}\n\n`;
    let editor;
    if ($(this).hasClass('quote-reply-diff')) {
      const $replyBtn = $(this).closest('.comment-code-cloud').find('button.comment-form-reply');
      editor = await handleReply($replyBtn);
    } else {
      // for normal issue/comment page
      editor = getComboMarkdownEditor($('#comment-form .combo-markdown-editor'));
    }
    if (editor) {
      if (editor.value()) {
        editor.value(`${editor.value()}\n\n${content}`);
      } else {
        editor.value(content);
      }
      editor.focus();
      editor.moveCursorToEnd();
    }
  });
}
