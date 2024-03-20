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
      issueID: parseInt($(card).attr('data-issue')),
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
    filter: '[data-id="0"]',
    animation: 150,
    ghostClass: 'card-ghost',
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
      animation: 150,
      ghostClass: 'card-ghost',
      onAdd: moveIssue,
      onUpdate: moveIssue,
      delayOnTouchOnly: true,
      delay: 500,
    });
  }
}

export function initRepoProject() {
  if (!$('.repository.projects').length) {
    return;
  }

  const _promise = initRepoProjectSortable();

  $('.edit-project-column-modal').each(function () {
    const $projectHeader = $(this).closest('.project-column-header');
    const $projectTitleLabel = $projectHeader.find('.project-column-title');
    const $projectTitleInput = $(this).find('.project-column-title-input');
    const $projectColorInput = $(this).find('#new_project_column_color');
    const $boardColumn = $(this).closest('.project-column');

    const bgColor = $boardColumn[0].style.backgroundColor;
    if (bgColor) {
      setLabelColor($projectHeader, rgbToHex(bgColor));
    }

    $(this).find('.edit-project-column-button').on('click', async function (e) {
      e.preventDefault();

      try {
        await PUT($(this).data('url'), {
          data: {
            title: $projectTitleInput.val(),
            color: $projectColorInput.val(),
          },
        });
      } catch (error) {
        console.error(error);
      } finally {
        $projectTitleLabel.text($projectTitleInput.val());
        $projectTitleInput.closest('form').removeClass('dirty');
        if ($projectColorInput.val()) {
          setLabelColor($projectHeader, $projectColorInput.val());
        }
        $boardColumn.attr('style', `background: ${$projectColorInput.val()}!important`);
        $('.ui.modal').modal('hide');
      }
    });
  });

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
    const $deleteColumnModal = $(`${$(this).attr('data-modal')}`);
    const $deleteColumnButton = $deleteColumnModal.find('.actions > .ok.button');
    const deleteUrl = $(this).attr('data-url');

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
    label.removeClass('dark-label').addClass('light-label');
  } else {
    label.removeClass('light-label').addClass('dark-label');
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
