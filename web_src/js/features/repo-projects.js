import $ from 'jquery';
import {useLightTextOnBackground} from '../utils/color.js';
import tinycolor from 'tinycolor2';
import {createSortable} from '../modules/sortable.js';
import {POST, DELETE, PUT} from '../modules/fetch.js';

function updateIssueCount(cards) {
  const parent = cards.parentElement;
  const cnt = parent.getElementsByClassName('issue-card').length;
  parent.getElementsByClassName('project-column-issue-count')[0].textContent = cnt;
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
  const columnCards = to.getElementsByClassName('issue-card');
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
  let boardColumns = mainBoard.getElementsByClassName('project-column');
  createSortable(mainBoard, {
    group: 'project-column',
    draggable: '.project-column',
    handle: '.project-column-header',
    delayOnTouchOnly: true,
    delay: 500,
    onSort: async () => {
      boardColumns = mainBoard.getElementsByClassName('project-column');
      for (let i = 0; i < boardColumns.length; i++) {
        const column = boardColumns[i];
        if (parseInt($(column).data('sorting')) !== i) {
          try {
            await PUT($(column).data('url'), {
              data: {
                sorting: i,
                color: rgbToHex(window.getComputedStyle($(column)[0]).backgroundColor),
              },
            });
          } catch (error) {
            console.error(error);
          }
        }
      }
    },
  });

  for (const boardColumn of boardColumns) {
    const boardCardList = boardColumn.getElementsByClassName('cards')[0];
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

  for (const modal of document.getElementsByClassName('edit-project-column-modal')) {
    const projectHeader = modal.closest('.project-column-header');
    const projectTitleLabel = projectHeader?.querySelector('.project-column-title');
    const projectTitleInput = modal.querySelector('.project-column-title-input');
    const projectColorInput = modal.querySelector('#new_project_column_color');
    const boardColumn = modal.closest('.project-column');
    const bgColor = boardColumn?.style.backgroundColor;

    if (bgColor) {
      setLabelColor(projectHeader, rgbToHex(bgColor));
    }

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
        if (projectColorInput?.value) {
          setLabelColor(projectHeader, projectColorInput.value);
        }
        boardColumn.style = `background: ${projectColorInput.value} !important`;
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

function setLabelColor(label, color) {
  const {r, g, b} = tinycolor(color).toRgb();
  if (useLightTextOnBackground(r, g, b)) {
    label.classList.remove('dark-label');
    label.classList.add('light-label');
  } else {
    label.classList.remove('light-label');
    label.classList.add('dark-label');
  }
}

function rgbToHex(rgb) {
  rgb = rgb.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+).*\)$/);
  return `#${hex(rgb[1])}${hex(rgb[2])}${hex(rgb[3])}`;
}

function hex(x) {
  const hexDigits = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'];
  return Number.isNaN(x) ? '00' : hexDigits[(x - x % 16) / 16] + hexDigits[x % 16];
}
