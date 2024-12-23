import {contrastColor} from '../utils/color.ts';
import {createSortable} from '../modules/sortable.ts';
import {POST, request} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {queryElemChildren, queryElems} from '../utils/dom.ts';
import type {SortableEvent} from 'sortablejs';

function updateIssueCount(card: HTMLElement): void {
  const parent = card.parentElement;
  const count = parent.querySelectorAll('.issue-card').length;
  parent.querySelector('.project-column-issue-count').textContent = String(count);
}

async function moveIssue({item, from, to, oldIndex}: SortableEvent): Promise<void> {
  const columnCards = to.querySelectorAll('.issue-card');
  updateIssueCount(from);
  updateIssueCount(to);

  const columnSorting = {
    issues: Array.from(columnCards, (card, i) => ({
      issueID: parseInt(card.getAttribute('data-issue')),
      sorting: i,
    })),
  };

  try {
    await POST(`${to.getAttribute('data-url')}/move`, {
      data: columnSorting,
    });
  } catch (error) {
    console.error(error);
    from.insertBefore(item, from.children[oldIndex]);
  }
}

async function initRepoProjectSortable(): Promise<void> {
  // the HTML layout is: #project-board > .board > .project-column .cards > .issue-card
  const mainBoard = document.querySelector('#project-board > .board.sortable');
  let boardColumns = mainBoard.querySelectorAll<HTMLElement>('.project-column');
  createSortable(mainBoard, {
    group: 'project-column',
    draggable: '.project-column',
    handle: '.project-column-header',
    delayOnTouchOnly: true,
    delay: 500,
    onSort: async () => { // eslint-disable-line @typescript-eslint/no-misused-promises
      boardColumns = mainBoard.querySelectorAll<HTMLElement>('.project-column');

      const columnSorting = {
        columns: Array.from(boardColumns, (column, i) => ({
          columnID: parseInt(column.getAttribute('data-id')),
          sorting: i,
        })),
      };

      try {
        await POST(mainBoard.getAttribute('data-url'), {
          data: columnSorting,
        });
      } catch (error) {
        console.error(error);
      }
    },
  });

  for (const boardColumn of boardColumns) {
    const boardCardList = boardColumn.querySelector('.cards');
    createSortable(boardCardList, {
      group: 'shared',
      onAdd: moveIssue, // eslint-disable-line @typescript-eslint/no-misused-promises
      onUpdate: moveIssue, // eslint-disable-line @typescript-eslint/no-misused-promises
      delayOnTouchOnly: true,
      delay: 500,
    });
  }
}

function initRepoProjectColumnEdit(writableProjectBoard: Element): void {
  const elModal = document.querySelector<HTMLElement>('.ui.modal#project-column-modal-edit');
  const elForm = elModal.querySelector<HTMLFormElement>('form');

  const elColumnId = elForm.querySelector<HTMLInputElement>('input[name="id"]');
  const elColumnTitle = elForm.querySelector<HTMLInputElement>('input[name="title"]');
  const elColumnColor = elForm.querySelector<HTMLInputElement>('input[name="color"]');

  const attrDataColumnId = 'data-modal-project-column-id';
  const attrDataColumnTitle = 'data-modal-project-column-title-input';
  const attrDataColumnColor = 'data-modal-project-column-color-input';

  // the "new" button is not in project board, so need to query from document
  queryElems(document, '.show-project-column-modal-edit', (el) => {
    el.addEventListener('click', () => {
      elColumnId.value = el.getAttribute(attrDataColumnId);
      elColumnTitle.value = el.getAttribute(attrDataColumnTitle);
      elColumnColor.value = el.getAttribute(attrDataColumnColor);
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
      await request(formLink, {method: formMethod, data: formData});
      if (!columnId) {
        window.location.reload(); // newly added column, need to reload the page
        return;
      }
      fomanticQuery(elModal).modal('hide');

      // update the newly saved column title and color in the project board (to avoid reload)
      const elEditButton = writableProjectBoard.querySelector<HTMLButtonElement>(`.show-project-column-modal-edit[${attrDataColumnId}="${columnId}"]`);
      elEditButton.setAttribute(attrDataColumnTitle, elColumnTitle.value);
      elEditButton.setAttribute(attrDataColumnColor, elColumnColor.value);

      const elBoardColumn = writableProjectBoard.querySelector<HTMLElement>(`.project-column[data-id="${columnId}"]`);
      const elBoardColumnTitle = elBoardColumn.querySelector<HTMLElement>(`.project-column-title-text`);
      elBoardColumnTitle.textContent = elColumnTitle.value;
      if (elColumnColor.value) {
        const textColor = contrastColor(elColumnColor.value);
        elBoardColumn.style.setProperty('background', elColumnColor.value, 'important');
        elBoardColumn.style.setProperty('color', textColor, 'important');
        queryElemChildren<HTMLElement>(elBoardColumn, '.divider', (divider) => divider.style.color = textColor);
      } else {
        elBoardColumn.style.removeProperty('background');
        elBoardColumn.style.removeProperty('color');
        queryElemChildren<HTMLElement>(elBoardColumn, '.divider', (divider) => divider.style.removeProperty('color'));
      }
    } finally {
      elForm.classList.remove('is-loading');
    }
  });
}

export function initRepoProject(): void {
  const writableProjectBoard = document.querySelector('#project-board[data-project-borad-writable="true"]');
  if (!writableProjectBoard) return;

  initRepoProjectSortable(); // no await
  initRepoProjectColumnEdit(writableProjectBoard);
}
