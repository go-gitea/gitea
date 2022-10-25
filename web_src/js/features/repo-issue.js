import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import attachTribute from './tribute.js';
import {createCommentEasyMDE, getAttachedEasyMDE} from './comp/EasyMDE.js';
import {initEasyMDEImagePaste} from './comp/ImagePaste.js';
import {initCompMarkupContentPreviewTab} from './comp/MarkupContentPreview.js';
import {initTooltip, showTemporaryTooltip} from '../modules/tippy.js';

const {appSubUrl, csrfToken} = window.config;

export function initRepoIssueTimeTracking() {
  $(document).on('click', '.issue-add-time', () => {
    $('.issue-start-time-modal').modal({
      duration: 200,
      onApprove() {
        $('#add_time_manual_form').trigger('submit');
      },
    }).modal('show');
    $('.issue-start-time-modal input').on('keydown', (e) => {
      if ((e.keyCode || e.key) === 13) {
        $('#add_time_manual_form').trigger('submit');
      }
    });
  });
  $(document).on('click', '.issue-start-time, .issue-stop-time', () => {
    $('#toggle_stopwatch_form').trigger('submit');
  });
  $(document).on('click', '.issue-cancel-time', () => {
    $('#cancel_stopwatch_form').trigger('submit');
  });
  $(document).on('click', 'button.issue-delete-time', function () {
    const sel = `.issue-delete-time-modal[data-id="${$(this).data('id')}"]`;
    $(sel).modal({
      duration: 200,
      onApprove() {
        $(`${sel} form`).trigger('submit');
      },
    }).modal('show');
  });
}

function updateDeadline(deadlineString) {
  $('#deadline-err-invalid-date').hide();
  $('#deadline-loader').addClass('loading');

  let realDeadline = null;
  if (deadlineString !== '') {
    const newDate = Date.parse(deadlineString);

    if (Number.isNaN(newDate)) {
      $('#deadline-loader').removeClass('loading');
      $('#deadline-err-invalid-date').show();
      return false;
    }
    realDeadline = new Date(newDate);
  }

  $.ajax(`${$('#update-issue-deadline-form').attr('action')}`, {
    data: JSON.stringify({
      due_date: realDeadline,
    }),
    headers: {
      'X-Csrf-Token': csrfToken,
    },
    contentType: 'application/json',
    type: 'POST',
    success() {
      window.location.reload();
    },
    error() {
      $('#deadline-loader').removeClass('loading');
      $('#deadline-err-invalid-date').show();
    },
  });
}

export function initRepoIssueDue() {
  $(document).on('click', '.issue-due-edit', () => {
    $('#deadlineForm').fadeToggle(150);
  });
  $(document).on('click', '.issue-due-remove', () => {
    updateDeadline('');
  });
  $(document).on('submit', '.issue-due-form', () => {
    updateDeadline($('#deadlineDate').val());
    return false;
  });
}

export function initRepoIssueList() {
  const repolink = $('#repolink').val();
  const repoId = $('#repoId').val();
  const crossRepoSearch = $('#crossRepoSearch').val();
  const tp = $('#type').val();
  let issueSearchUrl = `${appSubUrl}/${repolink}/issues/search?q={query}&type=${tp}`;
  if (crossRepoSearch === 'true') {
    issueSearchUrl = `${appSubUrl}/issues/search?q={query}&priority_repo_id=${repoId}&type=${tp}`;
  }
  $('#new-dependency-drop-list')
    .dropdown({
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
              name: `#${issue.number} ${htmlEscape(issue.title)
              }<div class="text small dont-break-out">${htmlEscape(issue.repository.full_name)}</div>`,
              value: issue.id,
            });
          });
          return filteredResponse;
        },
        cache: false,
      },

      fullTextSearch: true,
    });

  function excludeLabel(item) {
    const href = $(item).attr('href');
    const id = $(item).data('label-id');

    const regStr = `labels=((?:-?[0-9]+%2c)*)(${id})((?:%2c-?[0-9]+)*)&`;
    const newStr = 'labels=$1-$2$3&';

    window.location = href.replace(new RegExp(regStr), newStr);
  }

  $('.menu a.label-filter-item').each(function () {
    $(this).on('click', function (e) {
      if (e.altKey) {
        e.preventDefault();
        excludeLabel(this);
      }
    });
  });

  $('.menu .ui.dropdown.label-filter').on('keydown', (e) => {
    if (e.altKey && e.keyCode === 13) {
      const selectedItems = $('.menu .ui.dropdown.label-filter .menu .item.selected');
      if (selectedItems.length > 0) {
        excludeLabel($(selectedItems[0]));
      }
    }
  });
}

export function initRepoIssueCommentDelete() {
  // Delete comment
  $(document).on('click', '.delete-comment', function () {
    const $this = $(this);
    if (window.confirm($this.data('locale'))) {
      $.post($this.data('url'), {
        _csrf: csrfToken,
      }).done(() => {
        const $conversationHolder = $this.closest('.conversation-holder');

        // Check if this was a pending comment.
        if ($conversationHolder.find('.pending-label').length) {
          const $counter = $('#review-box .review-comments-counter');
          let num = parseInt($counter.attr('data-pending-comment-number')) - 1 || 0;
          num = Math.max(num, 0);
          $counter.attr('data-pending-comment-number', num);
          $counter.text(num);
        }

        $(`#${$this.data('comment-id')}`).remove();
        if ($conversationHolder.length && !$conversationHolder.find('.comment').length) {
          const path = $conversationHolder.data('path');
          const side = $conversationHolder.data('side');
          const idx = $conversationHolder.data('idx');
          const lineType = $conversationHolder.closest('tr').data('line-type');
          if (lineType === 'same') {
            $(`[data-path="${path}"] a.add-code-comment[data-idx="${idx}"]`).removeClass('invisible');
          } else {
            $(`[data-path="${path}"] a.add-code-comment[data-side="${side}"][data-idx="${idx}"]`).removeClass('invisible');
          }
          $conversationHolder.remove();
        }
      });
    }
    return false;
  });
}

export function initRepoIssueDependencyDelete() {
  // Delete Issue dependency
  $(document).on('click', '.delete-dependency-button', (e) => {
    const id = e.currentTarget.getAttribute('data-id');
    const type = e.currentTarget.getAttribute('data-type');

    $('.remove-dependency').modal({
      closable: false,
      duration: 200,
      onApprove: () => {
        $('#removeDependencyID').val(id);
        $('#dependencyType').val(type);
        $('#removeDependencyForm').trigger('submit');
      },
    }).modal('show');
  });
}

export function initRepoIssueCodeCommentCancel() {
  // Cancel inline code comment
  $(document).on('click', '.cancel-code-comment', (e) => {
    const form = $(e.currentTarget).closest('form');
    if (form.length > 0 && form.hasClass('comment-form')) {
      form.addClass('hide');
      form.closest('.comment-code-cloud').find('button.comment-form-reply').show();
    } else {
      form.closest('.comment-code-cloud').remove();
    }
  });
}

export function initRepoIssueStatusButton() {
  // Change status
  const $statusButton = $('#status-button');
  $('#comment-form textarea').on('keyup', function () {
    const easyMDE = getAttachedEasyMDE(this);
    const value = easyMDE?.value() || $(this).val();
    $statusButton.text($statusButton.data(value.length === 0 ? 'status' : 'status-and-comment'));
  });
  $statusButton.on('click', () => {
    $('#status').val($statusButton.data('status-val'));
    $('#comment-form').trigger('submit');
  });
}

export function initRepoPullRequestUpdate() {
  // Pull Request update button
  const $pullUpdateButton = $('.update-button > button');
  $pullUpdateButton.on('click', function (e) {
    e.preventDefault();
    const $this = $(this);
    const redirect = $this.data('redirect');
    $this.addClass('loading');
    $.post($this.data('do'), {
      _csrf: csrfToken
    }).done((data) => {
      if (data.redirect) {
        window.location.href = data.redirect;
      } else if (redirect) {
        window.location.href = redirect;
      } else {
        window.location.reload();
      }
    });
  });

  $('.update-button > .dropdown').dropdown({
    onChange(_text, _value, $choice) {
      const $url = $choice.data('do');
      if ($url) {
        $pullUpdateButton.find('.button-text').text($choice.text());
        $pullUpdateButton.data('do', $url);
      }
    }
  });
}

export function initRepoPullRequestMergeInstruction() {
  $('.show-instruction').on('click', () => {
    $('.instruct-content').toggle();
  });
}

export function initRepoPullRequestAllowMaintainerEdit() {
  const $checkbox = $('#allow-edits-from-maintainers');
  if (!$checkbox.length) return;

  const promptTip = $checkbox.attr('data-prompt-tip');
  const promptError = $checkbox.attr('data-prompt-error');

  initTooltip($checkbox[0], {content: promptTip});
  $checkbox.checkbox({
    'onChange': () => {
      const checked = $checkbox.checkbox('is checked');
      let url = $checkbox.attr('data-url');
      url += '/set_allow_maintainer_edit';
      $checkbox.checkbox('set disabled');
      $.ajax({url, type: 'POST',
        data: {_csrf: csrfToken, allow_maintainer_edit: checked},
        error: () => {
          showTemporaryTooltip($checkbox[0], promptError);
        },
        complete: () => {
          $checkbox.checkbox('set enabled');
        },
      });
    },
  });
}

export function initRepoIssueReferenceRepositorySearch() {
  $('.issue_reference_repository_search')
    .dropdown({
      apiSettings: {
        url: `${appSubUrl}/repo/search?q={query}&limit=20`,
        onResponse(response) {
          const filteredResponse = {success: true, results: []};
          $.each(response.data, (_r, repo) => {
            filteredResponse.results.push({
              name: htmlEscape(repo.full_name),
              value: repo.full_name
            });
          });
          return filteredResponse;
        },
        cache: false,
      },
      onChange(_value, _text, $choice) {
        const $form = $choice.closest('form');
        $form.attr('action', `${appSubUrl}/${_text}/issues/new`);
      },
      fullTextSearch: true
    });
}


export function initRepoIssueWipTitle() {
  $('.title_wip_desc > a').on('click', (e) => {
    e.preventDefault();

    const $issueTitle = $('#issue_title');
    $issueTitle.focus();
    const value = $issueTitle.val().trim().toUpperCase();

    const wipPrefixes = $('.title_wip_desc').data('wip-prefixes');
    for (const prefix of wipPrefixes) {
      if (value.startsWith(prefix.toUpperCase())) {
        return;
      }
    }

    $issueTitle.val(`${wipPrefixes[0]} ${$issueTitle.val()}`);
  });
}

export async function updateIssuesMeta(url, action, issueIds, elementId) {
  return $.ajax({
    type: 'POST',
    url,
    data: {
      _csrf: csrfToken,
      action,
      issue_ids: issueIds,
      id: elementId,
    },
  });
}

export function initRepoIssueComments() {
  if ($('.repository.view.issue .timeline').length === 0) return;

  $('.re-request-review').on('click', function (e) {
    e.preventDefault();
    const url = $(this).data('update-url');
    const issueId = $(this).data('issue-id');
    const id = $(this).data('id');
    const isChecked = $(this).hasClass('checked');

    updateIssuesMeta(
      url,
      isChecked ? 'detach' : 'attach',
      issueId,
      id,
    ).then(() => window.location.reload());
  });

  $('.dismiss-review-btn').on('click', function (e) {
    e.preventDefault();
    const $this = $(this);
    const $dismissReviewModal = $this.next();
    $dismissReviewModal.modal('show');
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


function assignMenuAttributes(menu) {
  const id = Math.floor(Math.random() * Math.floor(1000000));
  menu.attr('data-write', menu.attr('data-write') + id);
  menu.attr('data-preview', menu.attr('data-preview') + id);
  menu.find('.item').each(function () {
    const tab = $(this).attr('data-tab') + id;
    $(this).attr('data-tab', tab);
  });
  menu.parent().find("*[data-tab='write']").attr('data-tab', `write${id}`);
  menu.parent().find("*[data-tab='preview']").attr('data-tab', `preview${id}`);
  initCompMarkupContentPreviewTab(menu.parent('.form'));
  return id;
}

export function initRepoPullRequestReview() {
  if (window.location.hash && window.location.hash.startsWith('#issuecomment-')) {
    const commentDiv = $(window.location.hash);
    if (commentDiv) {
      // get the name of the parent id
      const groupID = commentDiv.closest('div[id^="code-comments-"]').attr('id');
      if (groupID && groupID.startsWith('code-comments-')) {
        const id = groupID.slice(14);
        $(`#show-outdated-${id}`).addClass('hide');
        $(`#code-comments-${id}`).removeClass('hide');
        $(`#code-preview-${id}`).removeClass('hide');
        $(`#hide-outdated-${id}`).removeClass('hide');
        commentDiv[0].scrollIntoView();
      }
    }
  }

  $(document).on('click', '.show-outdated', function (e) {
    e.preventDefault();
    const id = $(this).data('comment');
    $(this).addClass('hide');
    $(`#code-comments-${id}`).removeClass('hide');
    $(`#code-preview-${id}`).removeClass('hide');
    $(`#hide-outdated-${id}`).removeClass('hide');
  });

  $(document).on('click', '.hide-outdated', function (e) {
    e.preventDefault();
    const id = $(this).data('comment');
    $(this).addClass('hide');
    $(`#code-comments-${id}`).addClass('hide');
    $(`#code-preview-${id}`).addClass('hide');
    $(`#show-outdated-${id}`).removeClass('hide');
  });

  $(document).on('click', 'button.comment-form-reply', async function (e) {
    e.preventDefault();

    $(this).hide();
    const form = $(this).closest('.comment-code-cloud').find('.comment-form');
    form.removeClass('hide');
    const $textarea = form.find('textarea');
    let easyMDE = getAttachedEasyMDE($textarea);
    if (!easyMDE) {
      await attachTribute($textarea.get(), {mentions: true, emoji: true});
      easyMDE = await createCommentEasyMDE($textarea);
    }
    $textarea.focus();
    easyMDE.codemirror.focus();
    assignMenuAttributes(form.find('.menu'));
  });

  const $reviewBox = $('.review-box');
  if ($reviewBox.length === 1) {
    (async () => {
      // the editor's height is too large in some cases, and the panel cannot be scrolled with page now because there is `.repository .diff-detail-box.sticky { position: sticky; }`
      // the temporary solution is to make the editor's height smaller (about 4 lines). GitHub also only show 4 lines for default. We can improve the UI (including Dropzone area) in future
      // EasyMDE's options can not handle minHeight & maxHeight together correctly, we have to set max-height for .CodeMirror-scroll in CSS.
      const $reviewTextarea = $reviewBox.find('textarea');
      const easyMDE = await createCommentEasyMDE($reviewTextarea, {minHeight: '80px'});
      initEasyMDEImagePaste(easyMDE, $reviewBox.find('.dropzone'));
    })();
  }

  // The following part is only for diff views
  if ($('.repository.pull.diff').length === 0) {
    return;
  }

  $('.btn-review').on('click', function (e) {
    e.preventDefault();
    $(this).closest('.dropdown').find('.menu').toggle('visible');
  }).closest('.dropdown').find('.close').on('click', function (e) {
    e.preventDefault();
    $(this).closest('.menu').toggle('visible');
  });

  $(document).on('click', 'a.add-code-comment', async function (e) {
    if ($(e.target).hasClass('btn-add-single')) return; // https://github.com/go-gitea/gitea/issues/4745
    e.preventDefault();

    const isSplit = $(this).closest('.code-diff').hasClass('code-diff-split');
    const side = $(this).data('side');
    const idx = $(this).data('idx');
    const path = $(this).closest('[data-path]').data('path');
    const tr = $(this).closest('tr');
    const lineType = tr.data('line-type');

    let ntr = tr.next();
    if (!ntr.hasClass('add-comment')) {
      ntr = $(`
        <tr class="add-comment" data-line-type="${lineType}">
          ${isSplit ? `
            <td class="lines-num"></td>
            <td class="lines-escape"></td>
            <td class="lines-type-marker"></td>
            <td class="add-comment-left" colspan="4"></td>
            <td class="lines-num"></td>
            <td class="lines-escape"></td>
            <td class="lines-type-marker"></td>
            <td class="add-comment-right" colspan="4"></td>
          ` : `
            <td class="lines-num"></td>
            <td class="lines-num"></td>
            <td class="lines-escape"></td>
            <td class="add-comment-left add-comment-right" colspan="5"></td>
          `}
        </tr>`);
      tr.after(ntr);
    }

    const td = ntr.find(`.add-comment-${side}`);
    let commentCloud = td.find('.comment-code-cloud');
    if (commentCloud.length === 0 && !ntr.find('button[name="is_review"]').length) {
      const data = await $.get($(this).closest('[data-new-comment-url]').data('new-comment-url'));
      td.html(data);
      commentCloud = td.find('.comment-code-cloud');
      assignMenuAttributes(commentCloud.find('.menu'));
      td.find("input[name='line']").val(idx);
      td.find("input[name='side']").val(side === 'left' ? 'previous' : 'proposed');
      td.find("input[name='path']").val(path);
      const $textarea = commentCloud.find('textarea');
      await attachTribute($textarea.get(), {mentions: true, emoji: true});
      const easyMDE = await createCommentEasyMDE($textarea);
      $textarea.focus();
      easyMDE.codemirror.focus();
    }
  });
}

export function initRepoIssueReferenceIssue() {
  // Reference issue
  $(document).on('click', '.reference-issue', function (event) {
    const $this = $(this);
    $this.closest('.dropdown').find('.menu').toggle('visible');

    const content = $(`#comment-${$this.data('target')}`).text();
    const poster = $this.data('poster-username');
    const reference = $this.data('reference');
    const $modal = $($this.data('modal'));
    $modal.find('textarea[name="content"]').val(`${content}\n\n_Originally posted by @${poster} in ${reference}_`);
    $modal.modal('show');

    event.preventDefault();
  });
}

export function initRepoIssueWipToggle() {
  // Toggle WIP
  $('.toggle-wip a, .toggle-wip button').on('click', async (e) => {
    e.preventDefault();
    const toggleWip = e.currentTarget.closest('.toggle-wip');
    const title = toggleWip.getAttribute('data-title');
    const wipPrefix = toggleWip.getAttribute('data-wip-prefix');
    const updateUrl = toggleWip.getAttribute('data-update-url');
    await $.post(updateUrl, {
      _csrf: csrfToken,
      title: title?.startsWith(wipPrefix) ? title.slice(wipPrefix.length).trim() : `${wipPrefix.trim()} ${title}`,
    });
    window.location.reload();
  });
}


export function initRepoIssueTitleEdit() {
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
        _csrf: csrfToken,
        target_branch: targetBranch
      }).done((data) => {
        $branchTarget.text(data.base_branch);
      }).always(() => {
        window.location.reload();
      });
    };

    const pullrequest_target_update_url = $(this).data('target-update-url');
    if ($editInput.val().length === 0 || $editInput.val() === $issueTitle.text()) {
      $editInput.val($issueTitle.text());
      pullrequest_targetbranch_change(pullrequest_target_update_url);
    } else {
      $.post($(this).data('update-url'), {
        _csrf: csrfToken,
        title: $editInput.val()
      }, (data) => {
        $editInput.val(data.title);
        $issueTitle.text(data.title);
        pullrequest_targetbranch_change(pullrequest_target_update_url);
        window.location.reload();
      });
    }
    return false;
  });
}

export function initRepoIssueBranchSelect() {
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
}
