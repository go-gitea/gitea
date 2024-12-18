import {contrastColor} from '../utils/color.ts';
import {createSortable} from '../modules/sortable.ts';
import {POST, DELETE, PUT} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

function updateIssueCount(card: HTMLElement): void {
  const parent = card.parentElement;
  const cnt = parent.querySelectorAll('.issue-card').length;
  parent.querySelectorAll('.project-column-issue-count')[0].textContent = String(cnt);
}

async function createNewColumn(url: string, columnTitleInput: HTMLInputElement, projectColorInput: HTMLInputElement): Promise<void> {
  try {
    await POST(url, {
      data: {
        title: columnTitleInput.value,
        color: projectColorInput.value,
      },
    });
  } catch (error) {
    console.error(error);
  } finally {
    columnTitleInput.closest('form').classList.remove('dirty');
    window.location.reload();
  }
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
  const els = document.querySelectorAll('#project-board > .board.sortable');
  if (!els.length) return;

  // the HTML layout is: #project-board > .board > .project-column .cards > .issue-card
  const mainBoard = els[0];
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
    const boardCardList = boardColumn.querySelectorAll('.cards')[0];
    createSortable(boardCardList, {
      group: 'shared',
      onAdd: moveIssue, // eslint-disable-line @typescript-eslint/no-misused-promises
      onUpdate: moveIssue, // eslint-disable-line @typescript-eslint/no-misused-promises
      delayOnTouchOnly: true,
      delay: 500,
    });
  }
}

export function initRepoProject(): void {
  if (!document.querySelector('.repository.projects')) {
    return;
  }

  initRepoProjectSortable(); // no await

  for (const modal of document.querySelectorAll<HTMLDivElement>('.edit-project-column-modal')) {
    const projectHeader = modal.closest<HTMLElement>('.project-column-header');
    const projectTitleLabel = projectHeader?.querySelector<HTMLElement>('.project-column-title-label');
    const projectTitleInput = modal.querySelector<HTMLInputElement>('.project-column-title-input');
    const projectColorInput = modal.querySelector<HTMLInputElement>('#new_project_column_color');
    const boardColumn = modal.closest<HTMLElement>('.project-column');
    modal.querySelector('.edit-project-column-button')?.addEventListener('click', async function (e) {
      e.preventDefault();
      try {
        await PUT(this.getAttribute('data-url'), {
          data: {
            title: projectTitleInput?.value,
            color: projectColorInput?.value,
          },
        });
      } catch (error) {
        console.error(error);
      } finally {
        projectTitleLabel.textContent = projectTitleInput?.value;
        projectTitleInput.closest('form')?.classList.remove('dirty');
        const dividers = boardColumn.querySelectorAll<HTMLElement>(':scope > .divider');
        if (projectColorInput.value) {
          const color = contrastColor(projectColorInput.value);
          boardColumn.style.setProperty('background', projectColorInput.value, 'important');
          boardColumn.style.setProperty('color', color, 'important');
          for (const divider of dividers) {
            divider.style.setProperty('color', color);
          }
        } else {
          boardColumn.style.removeProperty('background');
          boardColumn.style.removeProperty('color');
          for (const divider of dividers) {
            divider.style.removeProperty('color');
          }
        }
        fomanticQuery('.ui.modal').modal('hide');
      }
    });
  }

  for (const modal of document.querySelectorAll('.default-project-column-modal')) {
    const column = modal.closest('.project-column');
    const showBtn = column.querySelector('.default-project-column-show');
    const okBtn = modal.querySelector('.actions .ok.button');
    okBtn.addEventListener('click', async (e: MouseEvent) => {
      e.preventDefault();
      try {
        await POST(showBtn.getAttribute('data-url'));
      } catch (error) {
        console.error(error);
      } finally {
        window.location.reload();
      }
    });
  }

  for (const btn of document.querySelectorAll('.show-delete-project-column-modal')) {
    const okBtn = document.querySelector(`${btn.getAttribute('data-modal')} .actions .ok.button`);
    okBtn?.addEventListener('click', async (e: MouseEvent) => {
      e.preventDefault();
      try {
        await DELETE(btn.getAttribute('data-url'));
      } catch (error) {
        console.error(error);
      } finally {
        window.location.reload();
      }
    });
  }

  document.querySelector('#new_project_column_submit')?.addEventListener('click', async (e: MouseEvent & {target: HTMLButtonElement}) => {
    e.preventDefault();
    const columnTitleInput = document.querySelector<HTMLInputElement>('#new_project_column');
    const projectColorInput = document.querySelector<HTMLInputElement>('#new_project_column_color_picker');
    if (!columnTitleInput.value) return;
    const url = e.target.getAttribute('data-url');
    await createNewColumn(url, columnTitleInput, projectColorInput);
  });
}
