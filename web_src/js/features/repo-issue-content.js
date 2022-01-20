import {svg} from '../svg.js';

const {appSubUrl, csrfToken} = window.config;

let i18nTextEdited;
let i18nTextOptions;
let i18nTextDeleteFromHistory;
let i18nTextDeleteFromHistoryConfirm;

function showContentHistoryDetail(issueBaseUrl, commentId, historyId, itemTitleHtml) {
  let $dialog = $('.content-history-detail-dialog');
  if ($dialog.length) return;

  $dialog = $(`
<div class="ui modal content-history-detail-dialog">
  <i class="close icon inside"></i>
  <div class="header">
    ${itemTitleHtml}
    <div class="ui dropdown right dialog-header-options" style="display: none; margin-right: 50px;">
      ${i18nTextOptions} <i class="dropdown icon"></i>
      <div class="menu">
        <div class="item red text" data-option-item="delete">${i18nTextDeleteFromHistory}</div>
      </div>
    </div>
  </div>
  <!-- ".modal .content" style was polluted in "_base.less": "&.modal > .content"  -->
  <div class="scrolling content" style="text-align: left; min-height: 30vh;">
      <div class="ui loader active"></div>
  </div>
</div>`);
  $dialog.appendTo($('body'));
  $dialog.find('.dialog-header-options').dropdown({
    showOnFocus: false,
    allowReselection: true,
    onChange(_value, _text, $item) {
      const optionItem = $item.data('option-item');
      if (optionItem === 'delete') {
        if (window.confirm(i18nTextDeleteFromHistoryConfirm)) {
          $.post(`${issueBaseUrl}/content-history/soft-delete?comment_id=${commentId}&history_id=${historyId}`, {
            _csrf: csrfToken,
          }).done((resp) => {
            if (resp.ok) {
              $dialog.modal('hide');
            } else {
              alert(resp.message);
            }
          });
        }
      } else { // required by eslint
        window.alert(`unknown option item: ${optionItem}`);
      }
    },
    onHide() {
      $(this).dropdown('clear', true);
    }
  });
  $dialog.modal({
    onShow() {
      $.ajax({
        url: `${issueBaseUrl}/content-history/detail?comment_id=${commentId}&history_id=${historyId}`,
        data: {
          _csrf: csrfToken,
        },
      }).done((resp) => {
        $dialog.find('.content').html(resp.diffHtml);
        // there is only one option "item[data-option-item=delete]", so the dropdown can be entirely shown/hidden.
        if (resp.canSoftDelete) {
          $dialog.find('.dialog-header-options').show();
        }
      });
    },
    onHidden() {
      $dialog.remove();
    },
  }).modal('show');
}

function showContentHistoryMenu(issueBaseUrl, $item, commentId) {
  const $headerLeft = $item.find('.comment-header-left');
  const menuHtml = `
  <div class="ui pointing dropdown top left content-history-menu" data-comment-id="${commentId}">
    <a>&bull; ${i18nTextEdited} ${svg('octicon-triangle-down', 17)}</a>
    <div class="menu">
    </div>
  </div>`;

  $headerLeft.find(`.content-history-menu`).remove();
  $headerLeft.append($(menuHtml));
  $headerLeft.find('.dropdown').dropdown({
    action: 'hide',
    apiSettings: {
      cache: false,
      url: `${issueBaseUrl}/content-history/list?comment_id=${commentId}`,
    },
    saveRemoteData: false,
    onHide() {
      $(this).dropdown('change values', null);
    },
    onChange(value, itemHtml, $item) {
      if (value && !$item.find('[data-history-is-deleted=1]').length) {
        showContentHistoryDetail(issueBaseUrl, commentId, value, itemHtml);
      }
    },
  });
}

export function initRepoIssueContentHistory() {
  const issueIndex = $('#issueIndex').val();
  if (!issueIndex) return;

  const $itemIssue = $('.repository.issue .timeline-item.comment.first'); // issue(PR) main content
  const $comments = $('.repository.issue .comment-list .comment'); // includes: issue(PR) comments, review comments, code comments
  if (!$itemIssue.length && !$comments.length) return;

  const repoLink = $('#repolink').val();
  const issueBaseUrl = `${appSubUrl}/${repoLink}/issues/${issueIndex}`;

  $.ajax({
    url: `${issueBaseUrl}/content-history/overview`,
    data: {
      _csrf: csrfToken,
    },
  }).done((resp) => {
    i18nTextEdited = resp.i18n.textEdited;
    i18nTextDeleteFromHistory = resp.i18n.textDeleteFromHistory;
    i18nTextDeleteFromHistoryConfirm = resp.i18n.textDeleteFromHistoryConfirm;
    i18nTextOptions = resp.i18n.textOptions;

    if (resp.editedHistoryCountMap[0] && $itemIssue.length) {
      showContentHistoryMenu(issueBaseUrl, $itemIssue, '0');
    }
    for (const [commentId, _editedCount] of Object.entries(resp.editedHistoryCountMap)) {
      if (commentId === '0') continue;
      const $itemComment = $(`#issuecomment-${commentId}`);
      showContentHistoryMenu(issueBaseUrl, $itemComment, commentId);
    }
  });
}
