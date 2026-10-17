import {contrastColor} from '../utils/color.ts';
import {createSortable} from '../modules/sortable.ts';
import {POST} from '../modules/fetch.ts';
import {performFetchActionRequest} from '../modules/fetch-action.ts';
import {hideFomanticModal} from '../modules/fomantic/modal.ts';
import {addDelegatedEventListener, queryElemChildren, queryElems, toggleElem} from '../utils/dom.ts';
import {createConfirmModal, confirmModal} from './comp/ConfirmModal.ts';
import {html, htmlRaw} from '../utils/html.ts';
import type {SortableEvent} from 'sortablejs';
import {toggleFullScreen} from '../utils.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {localUserSettings} from '../modules/user-settings.ts';

function updateColumn(elColumn: Element): void {
  elColumn.querySelector('.project-column-issue-count')!.textContent = String(elColumn.querySelectorAll('.issue-card').length);
  elColumn.querySelector('.project-column-remove-issues')?.classList.toggle('disabled', !elColumn.querySelector('.issue-card-remove'));
}

async function removeIssuesFromProject(elTrigger: HTMLElement, elColumnCards: HTMLElement, elCards: HTMLElement[]): Promise<void> {
  if (!elCards.length) return;
  const elModal = createConfirmModal({header: elTrigger.getAttribute('data-confirm-header')!, confirmButtonColor: 'red'});
  const titles = elCards.map((card) => card.querySelector('.issue-card-title')!.textContent.trim());
  elModal.querySelector('.content')!.innerHTML = html`
    <p>${elTrigger.getAttribute('data-confirm-content')}</p>
    <ul class="tw-max-h-[40vh] tw-overflow-y-auto">${htmlRaw(titles.map((title) => html`<li>${title}</li>`).join(''))}</ul>
  `;
  if (!await confirmModal(elModal)) return;

  const url = `${elColumnCards.getAttribute('data-url')}/remove-issues`;
  const issueIDs = elCards.map((card) => parseInt(card.getAttribute('data-issue')!));
  const resp = await performFetchActionRequest(elColumnCards, {method: 'POST', url, data: {issueIDs}, loadingIndicator: '$this'});
  if (!resp) return;
  // issues moved or hidden since the page loaded are skipped, so their cards stay
  const {removedIssueIDs}: {removedIssueIDs: number[]} = await resp.json();
  for (const card of elCards) {
    if (removedIssueIDs.includes(parseInt(card.getAttribute('data-issue')!))) card.remove();
  }
  updateColumn(elColumnCards.closest('.project-column')!);
}

function initRepoProjectRemoveIssues(writableProjectBoard: Element): void {
  for (const elColumn of writableProjectBoard.querySelectorAll('.project-column')) updateColumn(elColumn);
  addDelegatedEventListener(writableProjectBoard, 'click', '.project-column-remove-issues', (el) => {
    const elColumnCards = el.closest('.project-column')!.querySelector<HTMLElement>('.cards')!;
    const elCards = Array.from(elColumnCards.querySelectorAll<HTMLElement>('.issue-card:has(.issue-card-remove)'));
    return removeIssuesFromProject(el, elColumnCards, elCards);
  });
  addDelegatedEventListener(writableProjectBoard, 'click', '.issue-card-remove', (el) => {
    // the card may have been dragged since the page loaded, so its column is resolved now
    const elCard = el.closest<HTMLElement>('.issue-card')!;
    return removeIssuesFromProject(el, elCard.closest<HTMLElement>('.cards')!, [elCard]);
  });
}

async function moveIssue({item, from, to, oldIndex}: SortableEvent): Promise<void> {
  const columnCards = to.querySelectorAll('.issue-card');
  updateColumn(from.closest('.project-column')!);
  updateColumn(to.closest('.project-column')!);

  const columnSorting = {
    issues: Array.from(columnCards, (card, i) => ({
      issueID: parseInt(card.getAttribute('data-issue')!),
      sorting: i,
    })),
  };

  try {
    const resp = await POST(`${to.getAttribute('data-url')}/move`, {
      data: columnSorting,
    });
    if (!resp.ok) throw new Error(`Invalid server response: ${resp.status}`);
  } catch (error) {
    console.error(error);
    if (oldIndex !== undefined) {
      from.insertBefore(item, from.children[oldIndex]);
      updateColumn(from.closest('.project-column')!);
      updateColumn(to.closest('.project-column')!);
    }
  }
}

function initRepoProjectSortable(): void {
  // the HTML layout is: #project-board.board > .project-column .cards > .issue-card
  const mainBoard = document.querySelector<HTMLElement>('#project-board')!;
  let boardColumns = mainBoard.querySelectorAll<HTMLElement>('.project-column');
  createSortable(mainBoard, {
    group: 'project-column',
    draggable: '.project-column',
    handle: '.project-column-header',
    delayOnTouchOnly: true,
    delay: 500,
    onSort: async () => { // eslint-disable-line @typescript-eslint/no-misused-promises -- Sortable ignores the returned promise, the body catches its own errors
      boardColumns = mainBoard.querySelectorAll<HTMLElement>('.project-column');

      const columnSorting = {
        columns: Array.from(boardColumns, (column, i) => ({
          columnID: parseInt(column.getAttribute('data-id')!),
          sorting: i,
        })),
      };

      try {
        await POST(mainBoard.getAttribute('data-url')!, {
          data: columnSorting,
        });
      } catch (error) {
        console.error(error);
      }
    },
  });

  for (const boardColumn of boardColumns) {
    const boardCardList = boardColumn.querySelector<HTMLElement>('.cards')!;
    createSortable(boardCardList, {
      group: 'shared',
      onAdd: moveIssue, // eslint-disable-line @typescript-eslint/no-misused-promises -- Sortable ignores the returned promise, moveIssue catches its own errors
      onUpdate: moveIssue, // eslint-disable-line @typescript-eslint/no-misused-promises -- Sortable ignores the returned promise, moveIssue catches its own errors
      delayOnTouchOnly: true,
      delay: 500,
    });
  }
}

function initRepoProjectColumnEdit(writableProjectBoard: Element): void {
  const elModal = document.querySelector<HTMLElement>('.ui.modal#project-column-modal-edit')!;
  const elForm = elModal.querySelector<HTMLFormElement>('form')!;

  const elColumnId = elForm.querySelector<HTMLInputElement>('input[name="id"]')!;
  const elColumnTitle = elForm.querySelector<HTMLInputElement>('input[name="title"]')!;
  const elColumnColor = elForm.querySelector<HTMLInputElement>('input[name="color"]')!;

  const attrDataColumnId = 'data-modal-project-column-id';
  const attrDataColumnTitle = 'data-modal-project-column-title-input';
  const attrDataColumnColor = 'data-modal-project-column-color-input';

  // the "new" button is not in project board, so need to query from document
  queryElems(document, '.show-project-column-modal-edit', (el) => {
    el.addEventListener('click', () => {
      elColumnId.value = el.getAttribute(attrDataColumnId)!;
      elColumnTitle.value = el.getAttribute(attrDataColumnTitle)!;
      elColumnColor.value = el.getAttribute(attrDataColumnColor)!;
      elColumnColor.dispatchEvent(new Event('input', {bubbles: true})); // trigger the color picker
    });
  });

  elForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const columnId = elColumnId.value;
    const actionBaseLink = elForm.getAttribute('data-action-base-link');

    const formData = new FormData(elForm);
    const formLink = columnId ? `${actionBaseLink}/${columnId}` : `${actionBaseLink}/columns/new`;
    const formMethod = columnId ? 'PUT' : 'POST';

    try {
      elForm.classList.add('is-loading');
      const resp = await performFetchActionRequest(elForm, {url: formLink, method: formMethod, data: formData});
      if (!resp) return;
      if (!columnId) {
        window.location.reload(); // newly added column, need to reload the page
        return;
      }

      // update the newly saved column title and color in the project board (to avoid reload)
      const elEditButton = writableProjectBoard.querySelector<HTMLButtonElement>(`.show-project-column-modal-edit[${CSS.escape(attrDataColumnId)}="${CSS.escape(columnId)}"]`)!;
      elEditButton.setAttribute(attrDataColumnTitle, elColumnTitle.value);
      elEditButton.setAttribute(attrDataColumnColor, elColumnColor.value);

      const elBoardColumn = writableProjectBoard.querySelector<HTMLElement>(`.project-column[data-id="${CSS.escape(columnId)}"]`)!;
      const elBoardColumnTitle = elBoardColumn.querySelector<HTMLElement>(`.project-column-title-text`)!;
      elBoardColumnTitle.textContent = elColumnTitle.value;
      if (elColumnColor.value) {
        const textColor = contrastColor(elColumnColor.value);
        elBoardColumn.style.setProperty('background', elColumnColor.value, 'important');
        elBoardColumn.style.setProperty('color', textColor, 'important');
        queryElemChildren(elBoardColumn, '.divider', (divider: HTMLElement) => divider.style.color = textColor);
      } else {
        elBoardColumn.style.removeProperty('background');
        elBoardColumn.style.removeProperty('color');
        queryElemChildren(elBoardColumn, '.divider', (divider: HTMLElement) => divider.style.removeProperty('color'));
      }

      hideFomanticModal(elModal);
    } finally {
      elForm.classList.remove('is-loading');
    }
  });
}

function initRepoProjectToggleFullScreen(elProjectsView: HTMLElement): void {
  const enterFullscreenBtn = document.querySelector('.screen-full');
  const exitFullscreenBtn = document.querySelector('.screen-normal');
  if (!enterFullscreenBtn || !exitFullscreenBtn) return;

  const settingKey = 'projects-view-options';
  type ProjectsViewOptions = {
    fullScreen: boolean;
  };
  const opts = localUserSettings.getJsonObject<ProjectsViewOptions>(settingKey, {fullScreen: false});
  const toggleFullscreenState = (isFullScreen: boolean) => {
    toggleFullScreen(elProjectsView, isFullScreen);
    toggleElem(enterFullscreenBtn, !isFullScreen);
    toggleElem(exitFullscreenBtn, isFullScreen);

    opts.fullScreen = isFullScreen;
    localUserSettings.setJsonObject(settingKey, opts);
  };

  enterFullscreenBtn.addEventListener('click', () => toggleFullscreenState(true));
  exitFullscreenBtn.addEventListener('click', () => toggleFullscreenState(false));
  if (opts.fullScreen) {
    // a temporary solution to remember the full screen state, not perfect,
    // just make UX better than before, especially for users who need to change the label filter frequently and want to keep full screen mode.
    toggleFullscreenState(true);
  }
}

export function initRepoProjectsView(): void {
  registerGlobalInitFunc('initRepoProjectsView', (elProjectsView) => {
    initRepoProjectToggleFullScreen(elProjectsView);

    const writableProjectBoard = document.querySelector('#project-board[data-project-board-writable="true"]');
    if (!writableProjectBoard) return;

    initRepoProjectSortable();
    initRepoProjectColumnEdit(writableProjectBoard);
    initRepoProjectRemoveIssues(writableProjectBoard);
  });
}
