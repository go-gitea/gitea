import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
import {GET} from '../modules/fetch.ts';

async function loadBranchesAndTags(area: Element, loadingButton: Element) {
  loadingButton.classList.add('disabled');
  try {
    const res = await GET(loadingButton.getAttribute('data-fetch-url'));
    const data = await res.json();
    hideElem(loadingButton);
    addTags(area, data.tags);
    addBranches(area, data.branches, data.default_branch);
    showElem(area.querySelectorAll('.branch-and-tag-detail'));
  } finally {
    loadingButton.classList.remove('disabled');
  }
}

function addTags(area: Element, tags: Array<Record<string, any>>) {
  const tagArea = area.querySelector('.tag-area');
  toggleElem(tagArea.parentElement, tags.length > 0);
  for (const tag of tags) {
    addLink(tagArea, tag.web_link, tag.name);
  }
}

function addBranches(area: Element, branches: Array<Record<string, any>>, defaultBranch: string) {
  const defaultBranchTooltip = area.getAttribute('data-text-default-branch-tooltip');
  const branchArea = area.querySelector('.branch-area');
  toggleElem(branchArea.parentElement, branches.length > 0);
  for (const branch of branches) {
    const tooltip = defaultBranch === branch.name ? defaultBranchTooltip : null;
    addLink(branchArea, branch.web_link, branch.name, tooltip);
  }
}

function addLink(parent: Element, href: string, text: string, tooltip?: string) {
  const link = document.createElement('a');
  link.classList.add('muted', 'tw-px-1');
  link.href = href;
  link.textContent = text;
  if (tooltip) {
    link.classList.add('tw-border', 'tw-border-secondary', 'tw-rounded');
    link.setAttribute('data-tooltip-content', tooltip);
  }
  parent.append(link);
}

export function initRepoDiffCommitBranchesAndTags() {
  for (const area of document.querySelectorAll('.branch-and-tag-area')) {
    const btn = area.querySelector('.load-branches-and-tags');
    btn.addEventListener('click', () => loadBranchesAndTags(area, btn));
  }
}
