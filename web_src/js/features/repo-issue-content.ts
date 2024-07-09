import $ from 'jquery';
import {svg} from '../svg.ts';
import {showErrorToast} from '../modules/toast.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showElem} from '../utils/dom.ts';

const {appSubUrl} = window.config;
let i18nTextEdited;
let i18nTextOptions;
let i18nTextDeleteFromHistory;
let i18nTextDeleteFromHistoryConfirm;

function showContentHistoryDetail(issueBaseUrl, commentId, historyId, itemTitleHtml) {
  let $dialog = $('.content-history-detail-dialog');
  if ($dialog.length) return;

  $dialog = $(`
<div class="ui modal content-history-detail-dialog">
  ${svg('octicon-x', 16, 'close icon inside')}
  <div class="header tw-flex tw-items-center tw-justify-between">
    <div>${itemTitleHtml}</div>
    <div class="ui dropdown dialog-header-options tw-mr-8 tw-hidden">
      ${i18nTextOptions}
      ${svg('octicon-triangle-down', 14, 'dropdown icon')}
      <div class="menu">
        <div class="item red text" data-option-item="delete">${i18nTextDeleteFromHistory}</div>
      </div>
    </div>
  </div>
  <div class="comment-diff-data is-loading"></div>
</div>`);
  $dialog.appendTo($('body'));
  $dialog.find('.dialog-header-options').dropdown({
    showOnFocus: false,
    allowReselection: true,
    async onChange(_value, _text, $item) {
      const optionItem = $item.data('option-item');
      if (optionItem === 'delete') {
        if (window.confirm(i18nTextDeleteFromHistoryConfirm)) {
          try {
            const params = new URLSearchParams();
            params.append('comment_id', commentId);
            params.append('history_id', historyId);

            const response = await POST(`${issueBaseUrl}/content-history/soft-delete?${params.toString()}`);
            const resp = await response.json();

            if (resp.ok) {
              $dialog.modal('hide');
            } else {
              showErrorToast(resp.message);
            }
          } catch (error) {
            console.error('Error:', error);
            showErrorToast('An error occurred while deleting the history.');
          }
        }
      } else { // required by eslint
        showErrorToast(`unknown option item: ${optionItem}`);
      }
    },
    onHide() {
      $(this).dropdown('clear', true);
    },
  });
  $dialog.modal({
    async onShow() {
      try {
        const params = new URLSearchParams();
        params.append('comment_id', commentId);
        params.append('history_id', historyId);

        const url = `${issueBaseUrl}/content-history/detail?${params.toString()}`;
        const response = await GET(url);
        const resp = await response.json();

        const commentDiffData = $dialog.find('.comment-diff-data')[0];
        commentDiffData?.classList.remove('is-loading');
        commentDiffData.innerHTML = resp.diffHtml;
        // there is only one option "item[data-option-item=delete]", so the dropdown can be entirely shown/hidden.
        if (resp.canSoftDelete) {
          showElem($dialog.find('.dialog-header-options'));
        }
      } catch (error) {
        console.error('Error:', error);
      }
    },
    onHidden() {
      $dialog.remove();
    },
  }).modal('show');
}

function showContentHistoryMenu(issueBaseUrl, $item, commentId) {
  const $headerLeft = $item.find('.comment-header-left');
  const menuHtml = `
  <div class="ui dropdown interact-fg content-history-menu" data-comment-id="${commentId}">
    &bull; ${i18nTextEdited}${svg('octicon-triangle-down', 14, 'dropdown icon')}
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

export async function initRepoIssueContentHistory() {
  const issueIndex = $('#issueIndex').val();
  if (!issueIndex) return;

  const $itemIssue = $('.repository.issue .timeline-item.comment.first'); // issue(PR) main content
  const $comments = $('.repository.issue .comment-list .comment'); // includes: issue(PR) comments, review comments, code comments
  if (!$itemIssue.length && !$comments.length) return;

  const repoLink = $('#repolink').val();
  const issueBaseUrl = `${appSubUrl}/${repoLink}/issues/${issueIndex}`;

  try {
    const response = await GET(`${issueBaseUrl}/content-history/overview`);
    const resp = await response.json();

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
  } catch (error) {
    console.error('Error:', error);
  }
}
