import {contrastColor} from '../utils/color.ts';
import {createSortable} from '../modules/sortable.ts';
import {POST} from '../modules/fetch.ts';
import {performFetchActionRequest} from '../modules/fetch-action.ts';
import {hideFomanticModal} from '../modules/fomantic/modal.ts';
import {queryElemChildren, queryElems, toggleElem} from '../utils/dom.ts';
import type {SortableEvent} from 'sortablejs';
import {toggleFullScreen} from '../utils.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {localUserSettings} from '../modules/user-settings.ts';
import {moveWithinColumn} from './project-swimlanes.ts';

function updateIssueCount(card: HTMLElement): void {
  const parent = card.parentElement!;
  const count = parent.querySelectorAll('.issue-card').length;
  parent.querySelector('.project-column-issue-count')!.textContent = String(count);
}

async function moveIssue({item, from, to, oldIndex}: SortableEvent): Promise<void> {
  const columnCards = to.querySelectorAll('.issue-card');
  updateIssueCount(from);
  updateIssueCount(to);

  const columnSorting = {
    issues: Array.from(columnCards, (card, i) => ({
      issueID: parseInt(card.getAttribute('data-issue')!),
      sorting: i,
    })),
  };

  try {
    await POST(`${to.getAttribute('data-url')}/move`, {
      data: columnSorting,
    });
  } catch (error) {
    console.error(error);
    if (oldIndex !== undefined) {
      from.insertBefore(item, from.children[oldIndex]);
    }
  }
}

async function initRepoProjectSortable(): Promise<void> {
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

async function moveSwimlaneIssue({item, from, to, oldIndex}: SortableEvent): Promise<void> {
  const board = to.closest<HTMLElement>('#project-board')!;
  const columnID = to.getAttribute('data-board')!;
  const header = board.querySelector(`.project-column[data-id="${CSS.escape(columnID)}"]`)!;
  const column = JSON.parse(header.getAttribute('data-issue-order')!) as number[];
  const lane = Array.from(to.querySelectorAll('.issue-card'), (card) => Number(card.getAttribute('data-issue')));
  const issueID = Number(item.getAttribute('data-issue'));
  const order = moveWithinColumn(column, lane, issueID);
  board.inert = true;
  let saved = false;
  try {
    const response = await performFetchActionRequest(to, {
      url: `${to.getAttribute('data-url')}/move`,
      method: 'POST',
      data: {issues: order.map((id, sorting) => ({issueID: id, sorting}))},
    });
    if (response) {
      saved = true;
      window.location.reload(); // Refresh every copy of multi-label cards.
    }
  } finally {
    if (!saved) {
      if (oldIndex !== undefined) {
        item.remove();
        from.insertBefore(item, from.children.item(oldIndex));
      }
      board.inert = false;
    }
  }
}

function initProjectSwimlaneSortable(board: Element): void {
  for (const cards of board.querySelectorAll<HTMLElement>('.project-swimlane .cards')) {
    createSortable(cards, {
      group: `project-label-${cards.getAttribute('data-label-id')}`,
      onAdd: (event) => {
        moveSwimlaneIssue(event);
      },
      onUpdate: (event) => {
        moveSwimlaneIssue(event);
      },
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
      if (!columnId || writableProjectBoard.classList.contains('project-board-swimlanes')) {
        window.location.reload();
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

    if (writableProjectBoard.classList.contains('project-board-swimlanes')) {
      initProjectSwimlaneSortable(writableProjectBoard);
    } else {
      initRepoProjectSortable(); // no await
    }
    initRepoProjectColumnEdit(writableProjectBoard);
  });
}
