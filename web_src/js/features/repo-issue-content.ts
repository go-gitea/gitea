import {svg} from '../svg.ts';
import {showErrorToast} from '../modules/toast.ts';
import {GET, POST} from '../modules/fetch.ts';
import {createElementFromHTML, showElem} from '../utils/dom.ts';
import {parseIssuePageInfo} from '../utils.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

let i18nTextEdited: string;
let i18nTextOptions: string;
let i18nTextDeleteFromHistory: string;
let i18nTextDeleteFromHistoryConfirm: string;

function showContentHistoryDetail(issueBaseUrl: string, commentId: string, historyId: string, itemTitleHtml: string) {
  const elDetailDialog = createElementFromHTML(`
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
  document.body.append(elDetailDialog);
  const elOptionsDropdown = elDetailDialog.querySelector('.ui.dropdown.dialog-header-options');
  const $fomanticDialog = fomanticQuery(elDetailDialog);
  const $fomanticDropdownOptions = fomanticQuery(elOptionsDropdown);
  $fomanticDropdownOptions.dropdown({
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
              $fomanticDialog.modal('hide');
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
      $fomanticDropdownOptions.dropdown('clear', true);
    },
  });
  $fomanticDialog.modal({
    async onShow() {
      try {
        const params = new URLSearchParams();
        params.append('comment_id', commentId);
        params.append('history_id', historyId);

        const url = `${issueBaseUrl}/content-history/detail?${params.toString()}`;
        const response = await GET(url);
        const resp = await response.json();

        const commentDiffData = elDetailDialog.querySelector('.comment-diff-data');
        commentDiffData.classList.remove('is-loading');
        commentDiffData.innerHTML = resp.diffHtml;
        // there is only one option "item[data-option-item=delete]", so the dropdown can be entirely shown/hidden.
        if (resp.canSoftDelete) {
          showElem(elOptionsDropdown);
        }
      } catch (error) {
        console.error('Error:', error);
      }
    },
    onHidden() {
      $fomanticDialog.remove();
    },
  }).modal('show');
}

function showContentHistoryMenu(issueBaseUrl: string, elCommentItem: Element, commentId: string) {
  const elHeaderLeft = elCommentItem.querySelector('.comment-header-left');
  const menuHtml = `
  <div class="ui dropdown interact-fg content-history-menu" data-comment-id="${commentId}">
    &bull; ${i18nTextEdited}${svg('octicon-triangle-down', 14, 'dropdown icon')}
    <div class="menu">
    </div>
  </div>`;

  elHeaderLeft.querySelector(`.ui.dropdown.content-history-menu`)?.remove(); // remove the old one if exists
  elHeaderLeft.append(createElementFromHTML(menuHtml));

  const elDropdown = elHeaderLeft.querySelector('.ui.dropdown.content-history-menu');
  const $fomanticDropdown = fomanticQuery(elDropdown);
  $fomanticDropdown.dropdown({
    action: 'hide',
    apiSettings: {
      cache: false,
      url: `${issueBaseUrl}/content-history/list?comment_id=${commentId}`,
    },
    saveRemoteData: false,
    onHide() {
      $fomanticDropdown.dropdown('change values', null);
    },
    onChange(value, itemHtml, $item) {
      if (value && !$item.find('[data-history-is-deleted=1]').length) {
        showContentHistoryDetail(issueBaseUrl, commentId, value, itemHtml);
      }
    },
  });
}

export async function initRepoIssueContentHistory() {
  const issuePageInfo = parseIssuePageInfo();
  if (!issuePageInfo.issueNumber) return;

  const elIssueDescription = document.querySelector('.repository.issue .timeline-item.comment.first'); // issue(PR) main content
  const elComments = document.querySelectorAll('.repository.issue .comment-list .comment'); // includes: issue(PR) comments, review comments, code comments
  if (!elIssueDescription && !elComments.length) return;

  const issueBaseUrl = `${issuePageInfo.repoLink}/issues/${issuePageInfo.issueNumber}`;

  try {
    const response = await GET(`${issueBaseUrl}/content-history/overview`);
    const resp = await response.json();

    i18nTextEdited = resp.i18n.textEdited;
    i18nTextDeleteFromHistory = resp.i18n.textDeleteFromHistory;
    i18nTextDeleteFromHistoryConfirm = resp.i18n.textDeleteFromHistoryConfirm;
    i18nTextOptions = resp.i18n.textOptions;

    if (resp.editedHistoryCountMap[0] && elIssueDescription) {
      showContentHistoryMenu(issueBaseUrl, elIssueDescription, '0');
    }
    for (const [commentId, _editedCount] of Object.entries(resp.editedHistoryCountMap)) {
      if (commentId === '0') continue;
      const elIssueComment = document.querySelector(`#issuecomment-${commentId}`);
      if (elIssueComment) showContentHistoryMenu(issueBaseUrl, elIssueComment, commentId);
    }
  } catch (error) {
    console.error('Error:', error);
  }
}
