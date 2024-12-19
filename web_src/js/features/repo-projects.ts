import {contrastColor} from '../utils/color.ts';
import {createSortable} from '../modules/sortable.ts';
import {POST, request} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {queryElemChildren, queryElems} from '../utils/dom.ts';

function updateIssueCount(card: HTMLElement): void {
  const parent = card.parentElement;
  const cnt = parent.querySelectorAll('.issue-card').length;
  parent.querySelector('.project-column-issue-count').textContent = String(cnt);
}

async function moveIssue({item, from, to, oldIndex}: {item: HTMLElement, from: HTMLElement, to: HTMLElement, oldIndex: number}): Promise<void> {
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
  let boardColumns = mainBoard.querySelectorAll<HTMLDivElement>('.project-column');
  createSortable(mainBoard, {
    group: 'project-column',
    draggable: '.project-column',
    handle: '.project-column-header',
    delayOnTouchOnly: true,
    delay: 500,
    onSort: async () => { // eslint-disable-line @typescript-eslint/no-misused-promises
      boardColumns = mainBoard.querySelectorAll<HTMLDivElement>('.project-column');

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
  const elColumnEditModal = document.querySelector<HTMLFormElement>('.ui.modal#project-column-modal-edit');
  const elColumnEditForm = elColumnEditModal.querySelector<HTMLFormElement>('form');

  const elColumnEditId = elColumnEditForm.querySelector<HTMLInputElement>('input[name="id"]');
  const elColumnEditTitle = elColumnEditForm.querySelector<HTMLInputElement>('input[name="title"]');
  const elColumnEditColor = elColumnEditForm.querySelector<HTMLInputElement>('input[name="color"]');

  const dataAttrColumnId = 'data-modal-project-column-id';
  const dataAttrColumnTitle = 'data-modal-project-column-title-input';
  const dataAttrColumnColor = 'data-modal-project-column-color-input';

  // the "new" button is not in project board, so need to query from document
  queryElems(document, '.show-project-column-modal-edit', (el) => {
    el.addEventListener('click', () => {
      elColumnEditId.value = el.getAttribute(dataAttrColumnId);
      elColumnEditTitle.value = el.getAttribute(dataAttrColumnTitle);
      elColumnEditColor.value = el.getAttribute(dataAttrColumnColor);
      elColumnEditColor.dispatchEvent(new Event('input', {bubbles: true})); // trigger the color picker
    });
  });

  elColumnEditForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const columnId = elColumnEditId.value;
    const actionBaseLink = elColumnEditForm.getAttribute('data-action-base-link');

    const formData = new FormData(elColumnEditForm);
    const formLink = columnId ? `${actionBaseLink}/${columnId}` : `${actionBaseLink}/columns/new`;
    const formMethod = columnId ? 'PUT' : 'POST';

    try {
      elColumnEditForm.classList.add('is-loading');
      await request(formLink, {method: formMethod, data: formData});
      if (!columnId) {
        window.location.reload(); // newly added column, need to reload the page
        return;
      }
      fomanticQuery(elColumnEditModal).modal('hide');

      // update the newly saved column title and color in the project board (to avoid reload)
      const elEditButton = writableProjectBoard.querySelector<HTMLButtonElement>(`.show-project-column-modal-edit[${dataAttrColumnId}="${columnId}"]`);
      elEditButton.setAttribute(dataAttrColumnTitle, elColumnEditTitle.value);
      elEditButton.setAttribute(dataAttrColumnColor, elColumnEditColor.value);

      const elBoardColumn = writableProjectBoard.querySelector<HTMLElement>(`.project-column[data-id="${columnId}"]`);
      const elBoardColumnTitle = elBoardColumn.querySelector<HTMLElement>(`.project-column-title-text`);
      elBoardColumnTitle.textContent = elColumnEditTitle.value;
      if (elColumnEditColor.value) {
        const textColor = contrastColor(elColumnEditColor.value);
        elBoardColumn.style.setProperty('background', elColumnEditColor.value, 'important');
        elBoardColumn.style.setProperty('color', textColor, 'important');
        queryElemChildren<HTMLElement>(elBoardColumn, '.divider', (divider) => divider.style.color = textColor);
      } else {
        elBoardColumn.style.removeProperty('background');
        elBoardColumn.style.removeProperty('color');
        queryElemChildren<HTMLElement>(elBoardColumn, '.divider', (divider) => divider.style.removeProperty('color'));
      }
    } finally {
      elColumnEditForm.classList.remove('is-loading');
    }
  });
}

export function initRepoProject(): void {
  const writableProjectBoard = document.querySelector('#project-board[data-project-borad-writable="true"]');
  if (!writableProjectBoard) return;

  initRepoProjectSortable(); // no await
  initRepoProjectColumnEdit(writableProjectBoard);
}
