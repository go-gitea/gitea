import {contrastColor} from '../utils/color.ts';
import {createSortable} from '../modules/sortable.ts';
import {POST, request, GET} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {queryElemChildren, queryElems, toggleElem} from '../utils/dom.ts';
import {html} from '../utils/html.ts';
import type {SortableEvent} from 'sortablejs';
import {toggleFullScreen} from '../utils.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {localUserSettings} from '../modules/user-settings.ts';

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
    onSort: async () => { // eslint-disable-line @typescript-eslint/no-misused-promises
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
      onAdd: moveIssue, // eslint-disable-line @typescript-eslint/no-misused-promises
      onUpdate: moveIssue, // eslint-disable-line @typescript-eslint/no-misused-promises
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
      await request(formLink, {method: formMethod, data: formData});
      if (!columnId) {
        window.location.reload(); // newly added column, need to reload the page
        return;
      }

      // update the newly saved column title and color in the project board (to avoid reload)
      const elEditButton = writableProjectBoard.querySelector<HTMLButtonElement>(`.show-project-column-modal-edit[${attrDataColumnId}="${columnId}"]`)!;
      elEditButton.setAttribute(attrDataColumnTitle, elColumnTitle.value);
      elEditButton.setAttribute(attrDataColumnColor, elColumnColor.value);

      const elBoardColumn = writableProjectBoard.querySelector<HTMLElement>(`.project-column[data-id="${columnId}"]`)!;
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

      fomanticQuery(elModal).modal('hide');
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

    initRepoProjectSortable(); // no await
    initRepoProjectColumnEdit(writableProjectBoard);
    initRepoProjectAddIssueModal();
    initRepoProjectAddPullModal();
    initRepoProjectUnbindButton();
  });
}

function initRepoProjectAddIssueModal(): void {
  queryElems(document, '.show-add-issue-modal', (el) => {
    el.addEventListener('click', () => {
      const columnId = el.getAttribute('data-modal-column-id');
      const columnIdInput = document.querySelector<HTMLInputElement>('#add-issue-column-id');
      if (columnId && columnIdInput) {
        columnIdInput.value = columnId;
      }

      const issueRepoSelect = document.querySelector<HTMLElement>('#issue-repo');
      if (issueRepoSelect && (issueRepoSelect as HTMLInputElement).type === 'hidden') {
        issueRepoSelect.dispatchEvent(new Event('change'));
      }
    });
  });

  const issueRepoSelect = document.querySelector<HTMLElement>('#issue-repo');
  if (issueRepoSelect) {
    issueRepoSelect.addEventListener('change', async () => {
      const issueSelect = document.querySelector<HTMLSelectElement>('#issue-number');
      if (!issueSelect) return;

      const repoSelect = issueRepoSelect as HTMLSelectElement;
      const repoInput = issueRepoSelect as HTMLInputElement;

      if (!repoSelect.value && !repoInput.value) {
        issueSelect.disabled = true;
        issueSelect.innerHTML = html`<option value="">Choose an issue</option>`;
        return;
      }

      let owner: string;
      let name: string;
      if (repoInput.type === 'hidden') {
        owner = repoInput.getAttribute('data-owner') || '';
        name = repoInput.getAttribute('data-name') || '';
      } else {
        const selectedOption = repoSelect.options[repoSelect.selectedIndex];
        owner = selectedOption.getAttribute('data-owner') || '';
        name = selectedOption.getAttribute('data-name') || '';
      }

      issueSelect.disabled = true;
      issueSelect.innerHTML = html`<option value="">Loading...</option>`;

      try {
        const appSubUrl = (window as any).config?.appSubUrl || '';
        const response = await GET(`${appSubUrl}/api/v1/repos/${owner}/${name}/issues?state=open&type=issues`);
        const issues = await response.json();

        issueSelect.innerHTML = html`<option value="">Choose an issue</option>`;
        for (const issue of issues) {
          const option = document.createElement('option');
          option.value = issue.number;
          option.textContent = `#${issue.number} - ${issue.title}`;
          issueSelect.append(option);
        }
        issueSelect.disabled = false;
      } catch (error: any) {
        console.error('Failed to load issues:', error);
        // For 404 errors (e.g., empty repository), show normal select prompt
        issueSelect.innerHTML = html`<option value="">Choose an issue</option>`;
        issueSelect.disabled = false;
      }
    });
  }
}

function initRepoProjectAddPullModal(): void {
  queryElems(document, '.show-add-pull-modal', (el) => {
    el.addEventListener('click', () => {
      const columnId = el.getAttribute('data-modal-column-id');
      const columnIdInput = document.querySelector<HTMLInputElement>('#add-pull-column-id');
      if (columnId && columnIdInput) {
        columnIdInput.value = columnId;
      }

      const pullRepoSelect = document.querySelector<HTMLElement>('#pull-repo');
      if (pullRepoSelect && (pullRepoSelect as HTMLInputElement).type === 'hidden') {
        pullRepoSelect.dispatchEvent(new Event('change'));
      }
    });
  });

  const pullRepoSelect = document.querySelector<HTMLElement>('#pull-repo');
  if (pullRepoSelect) {
    pullRepoSelect.addEventListener('change', async () => {
      const pullSelect = document.querySelector<HTMLSelectElement>('#pull-number');
      if (!pullSelect) return;

      const repoSelect = pullRepoSelect as HTMLSelectElement;
      const repoInput = pullRepoSelect as HTMLInputElement;

      if (!repoSelect.value && !repoInput.value) {
        pullSelect.disabled = true;
        pullSelect.innerHTML = html`<option value="">Choose a pull request</option>`;
        return;
      }

      let owner: string;
      let name: string;
      if (repoInput.type === 'hidden') {
        owner = repoInput.getAttribute('data-owner') || '';
        name = repoInput.getAttribute('data-name') || '';
      } else {
        const selectedOption = repoSelect.options[repoSelect.selectedIndex];
        owner = selectedOption.getAttribute('data-owner') || '';
        name = selectedOption.getAttribute('data-name') || '';
      }

      pullSelect.disabled = true;
      pullSelect.innerHTML = html`<option value="">Loading...</option>`;

      try {
        const appSubUrl = (window as any).config?.appSubUrl || '';
        const response = await GET(`${appSubUrl}/api/v1/repos/${owner}/${name}/pulls?state=open`);
        const pulls = await response.json();

        pullSelect.innerHTML = html`<option value="">Choose a pull request</option>`;
        for (const pull of pulls) {
          const option = document.createElement('option');
          option.value = pull.number;
          option.textContent = `#${pull.number} - ${pull.title}`;
          pullSelect.append(option);
        }
        pullSelect.disabled = false;
      } catch (error: any) {
        console.error('Failed to load pull requests:', error);
        // For 404 errors (e.g., empty repository), show normal select prompt
        pullSelect.innerHTML = html`<option value="">Choose a pull request</option>`;
        pullSelect.disabled = false;
      }
    });
  }
}

function initRepoProjectUnbindButton(): void {
  queryElems(document, '.issue-card-unbind', (el) => {
    el.addEventListener('click', async (e) => {
      e.preventDefault();
      e.stopPropagation();

      const id = el.getAttribute('data-id');
      const columnId = el.getAttribute('data-column-id');

      if (!confirm('Are you sure you want to remove this issue from the project?')) {
        return;
      }

      const csrfToken = document.querySelector('meta[name="_csrf"]')?.getAttribute('content') || '';

      try {
        const projectBoardUrl = document.querySelector('#project-board')?.getAttribute('data-url') || '';
        const projectLink = projectBoardUrl.replace(/\/move$/, '');
        const response = await POST(`${projectLink}/${columnId}/unbind-issue`, {
          data: new URLSearchParams({issue_id: id!}),
          headers: {'X-Csrf-Token': csrfToken},
        });

        if (response.ok) {
          const card = el.closest('.issue-card');
          const column = card?.closest('.project-column');

          card?.remove();

          if (column) {
            const countLabel = column.querySelector('.project-column-issue-count');
            if (countLabel) {
              const currentCount = parseInt(countLabel.textContent || '0');
              countLabel.textContent = String(currentCount - 1);
            }
          }
        } else {
          alert('An error occurred');
        }
      } catch (error) {
        console.error('Failed to unbind card:', error);
        alert('An error occurred');
      }
    });
  });
}
