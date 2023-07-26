import {showElem, toggleElem} from '../utils/dom.js';

async function loadBranchesAndTags(area, loadingButton) {
  loadingButton.classList.add('disabled');
  try {
    const res = await fetch(loadingButton.getAttribute('data-fetch-url'));
    const data = await res.json();
    loadingButton.classList.add('gt-hidden');
    addTags(area, data.tags);
    addBranches(area, data.branches, data.default_branch);
    showElem(area.querySelectorAll('.branch-and-tag-detail'));
  } finally {
    loadingButton.classList.remove('disabled');
  }
}

function addTags(area, tags) {
  toggleElem(area.querySelectorAll('.tag-area-parent'), tags.length > 0);
  const tagArea = area.querySelector('.tag-area');
  for (const tag of tags) {
    addLink(tagArea, tag.web_link, tag.name);
  }
}

function addBranches(area, branches, defaultBranch) {
  const defaultBranchTooltip = area.getAttribute('data-text-default-branch-tooltip');
  toggleElem(area.querySelectorAll('.branch-area-parent'), branches.length > 0);
  const branchArea = area.querySelector('.branch-area');
  for (const branch of branches) {
    const tooltip = defaultBranch === branch.name ? defaultBranchTooltip : null;
    addLink(branchArea, branch.web_link, branch.name, tooltip);
  }
}

function addLink(parent, href, text, tooltip) {
  const link = document.createElement('a');
  link.classList.add('muted', 'gt-px-2', 'gt-rounded');
  link.href = href;
  link.textContent = text;
  if (tooltip) {
    link.classList.add('gt-border-secondary');
    link.setAttribute('data-tooltip-content', tooltip);
  }
  parent.append(link);
}

export function initLoadBranchesAndTagsButton() {
  for (const area of document.querySelectorAll('.branch-and-tag-area')) {
    const loadButton = area.querySelector('.load-branches-and-tags');
    loadButton.addEventListener('click', () => loadBranchesAndTags(area, loadButton));
  }
}
