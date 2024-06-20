import $ from 'jquery';
import {contrastColor} from '../utils/color.js';
import {createSortable} from '../modules/sortable.js';
import {POST, DELETE, PUT} from '../modules/fetch.js';

function updateIssueCount(cards) {
  const parent = cards.parentElement;
  const cnt = parent.querySelectorAll('.issue-card').length;
  parent.querySelectorAll('.project-column-issue-count')[0].textContent = cnt;
}

async function createNewColumn(url, columnTitle, projectColorInput) {
  try {
    await POST(url, {
      data: {
        title: columnTitle.val(),
        color: projectColorInput.val(),
      },
    });
  } catch (error) {
    console.error(error);
  } finally {
    columnTitle.closest('form').removeClass('dirty');
    window.location.reload();
  }
}

async function moveIssue({item, from, to, oldIndex}) {
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

async function initRepoProjectSortable() {
  const els = document.querySelectorAll('#project-board > .board.sortable');
  if (!els.length) return;

  // the HTML layout is: #project-board > .board > .project-column .cards > .issue-card
  const mainBoard = els[0];
  let boardColumns = mainBoard.querySelectorAll('.project-column');
  createSortable(mainBoard, {
    group: 'project-column',
    draggable: '.project-column',
    handle: '.project-column-header',
    delayOnTouchOnly: true,
    delay: 500,
    onSort: async () => {
      boardColumns = mainBoard.querySelectorAll('.project-column');

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
      onAdd: moveIssue,
      onUpdate: moveIssue,
      delayOnTouchOnly: true,
      delay: 500,
    });
  }
}

export function initRepoProject() {
  if (!document.querySelector('.repository.projects')) {
    return;
  }

  const _promise = initRepoProjectSortable();

  for (const modal of document.querySelectorAll('.edit-project-column-modal')) {
    const projectHeader = modal.closest('.project-column-header');
    const projectTitleLabel = projectHeader?.querySelector('.project-column-title-label');
    const projectTitleInput = modal.querySelector('.project-column-title-input');
    const projectColorInput = modal.querySelector('#new_project_column_color');
    const boardColumn = modal.closest('.project-column');
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
        const dividers = boardColumn.querySelectorAll(':scope > .divider');
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
        $('.ui.modal').modal('hide');
      }
    });
  }

  $('.default-project-column-modal').each(function () {
    const $boardColumn = $(this).closest('.project-column');
    const $showButton = $($boardColumn).find('.default-project-column-show');
    const $commitButton = $(this).find('.actions > .ok.button');

    $($commitButton).on('click', async (e) => {
      e.preventDefault();

      try {
        await POST($($showButton).data('url'));
      } catch (error) {
        console.error(error);
      } finally {
        window.location.reload();
      }
    });
  });

  $('.show-delete-project-column-modal').each(function () {
    const $deleteColumnModal = $(`${this.getAttribute('data-modal')}`);
    const $deleteColumnButton = $deleteColumnModal.find('.actions > .ok.button');
    const deleteUrl = this.getAttribute('data-url');

    $deleteColumnButton.on('click', async (e) => {
      e.preventDefault();

      try {
        await DELETE(deleteUrl);
      } catch (error) {
        console.error(error);
      } finally {
        window.location.reload();
      }
    });
  });

  $('#new_project_column_submit').on('click', (e) => {
    e.preventDefault();
    const $columnTitle = $('#new_project_column');
    const $projectColorInput = $('#new_project_column_color_picker');
    if (!$columnTitle.val()) {
      return;
    }
    const url = e.target.getAttribute('data-url');
    createNewColumn(url, $columnTitle, $projectColorInput);
  });
}
