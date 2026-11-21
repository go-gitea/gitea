import {svgRaw} from '../svg.ts';
import {showErrorToast} from '../modules/toast.ts';
import {GET, POST} from '../modules/fetch.ts';
import {addDelegatedEventListener, createElementFromHTML, showElem} from '../utils/dom.ts';
import {parseIssuePageInfo} from '../utils.ts';
import {initDropdown} from '../modules/dropdown.ts';
import {hideModal, showModal} from '../modules/modal.ts';
import {confirmModal} from './comp/ConfirmModal.ts';
import {html, htmlRaw} from '../utils/html.ts';

let i18nTextEdited: string;
let i18nTextOptions: string;
let i18nTextDeleteFromHistory: string;
let i18nTextDeleteFromHistoryConfirm: string;

function showContentHistoryDetail(issueBaseUrl: string, commentId: string, historyId: string, itemTitleHtml: string) {
  const elDetailDialog = createElementFromHTML<HTMLDialogElement>(html`
    <dialog class="modal-dialog content-history-detail-dialog">
      <div class="header flex-left-right">
        <div>${htmlRaw(itemTitleHtml)}</div>
        <div class="dropdown-container dialog-header-options tw-mx-4 tw-hidden">
          <button type="button" class="dropdown-trigger btn">
            ${i18nTextOptions} ${svgRaw('octicon-triangle-down', 14)}
          </button>
          <div class="dropdown-popup">
            <button type="button" class="dropdown-item tw-text-red">${i18nTextDeleteFromHistory}</button>
          </div>
        </div>
        <button type="button" class="close-modal"></button>
      </div>
      <div class="comment-diff-data is-loading"></div>
    </dialog>
  `);
  const optionsDropdown = initDropdown(elDetailDialog.querySelector('.dialog-header-options')!);
  optionsDropdown.popup.querySelector('.dropdown-item')!.addEventListener('click', async (e) => {
    e.preventDefault();
    if (!await confirmModal({content: i18nTextDeleteFromHistoryConfirm, confirmButtonColor: 'red'})) return;
    try {
      const params = new URLSearchParams({comment_id: commentId, history_id: historyId});
      const response = await POST(`${issueBaseUrl}/content-history/soft-delete?${params.toString()}`);
      const resp = await response.json();
      if (resp.ok) {
        hideModal(elDetailDialog);
      } else {
        showErrorToast(resp.message);
      }
    } catch (error) {
      console.error('Error:', error);
      showErrorToast('An error occurred while deleting the history.');
    }
  });
  showModal(elDetailDialog, {
    async onShow() {
      try {
        const params = new URLSearchParams();
        params.append('comment_id', commentId);
        params.append('history_id', historyId);

        const url = `${issueBaseUrl}/content-history/detail?${params.toString()}`;
        const response = await GET(url);
        const resp = await response.json();

        const commentDiffData = elDetailDialog.querySelector('.comment-diff-data')!;
        commentDiffData.classList.remove('is-loading');
        commentDiffData.innerHTML = resp.diffHtml;
        if (resp.canSoftDelete) {
          showElem(optionsDropdown.container);
        }
      } catch (error) {
        console.error('Error:', error);
      }
    },
    onHidden() {
      elDetailDialog.remove();
    },
  });
}

function showContentHistoryMenu(issueBaseUrl: string, elCommentItem: Element, commentId: string) {
  const elHeaderLeft = elCommentItem.querySelector('.comment-header-left')!;
  const elDropdown = createElementFromHTML<HTMLElement>(html`
    <div class="dropdown-container content-history-menu tw-flex-shrink-0" data-comment-id="${commentId}">
      <button type="button" class="dropdown-trigger btn interact-fg">
        &bull; ${i18nTextEdited} ${svgRaw('octicon-triangle-down', 14)}
      </button>
      <div class="dropdown-popup"></div>
    </div>
  `);
  elHeaderLeft.querySelector('.content-history-menu')?.remove();
  elHeaderLeft.append(elDropdown);
  const dropdown = initDropdown(elDropdown, {
    async loadItems(_query, signal) {
      const response = await GET(`${issueBaseUrl}/content-history/list?comment_id=${commentId}`, {signal});
      const {results} = await response.json() as {results: Array<{name: string, value: number}>};
      return results.map(({name, value}) => {
        const item = createElementFromHTML(html`<div class="dropdown-item" data-value="${value}">${htmlRaw(name)}</div>`);
        if (item.querySelector('[data-history-is-deleted="1"]')) item.setAttribute('aria-disabled', 'true');
        return item;
      });
    },
  });
  addDelegatedEventListener(dropdown.popup, 'click', '.dropdown-item', (item) => {
    showContentHistoryDetail(issueBaseUrl, commentId, item.getAttribute('data-value')!, item.innerHTML);
  });
}

export async function initRepoIssueContentHistory() {
  const issuePageInfo = parseIssuePageInfo();
  if (!issuePageInfo.issueNumber) return;

  const elIssueDescription = document.querySelector('.repository.issue .timeline-item.comment.issue-content-comment'); // issue(PR) main content
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
    for (const commentId of Object.keys(resp.editedHistoryCountMap)) {
      if (commentId === '0') continue;
      const elIssueComment = document.querySelector(`#issuecomment-${commentId}`);
      if (elIssueComment) showContentHistoryMenu(issueBaseUrl, elIssueComment, commentId);
    }
  } catch (error) {
    console.error('Error:', error);
  }
}
