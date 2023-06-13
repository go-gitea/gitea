const {csrfToken} = window.config;

async function loadBranchesAndTags(loadingButton, addHere) {
  loadingButton.setAttribute('disabled', 'disabled');
  let res;
  try {
    res = await fetch(loadingButton.getAttribute('data-fetch-url'), {
      method: 'GET',
      headers: {'X-Csrf-Token': csrfToken},
    });
  } finally {
    loadingButton.removeAttribute('disabled');
  }

  if (!res.ok) {
    return;
  }

  const data = await res.json();
  showAreas('.branch-tag-area-divider');
  loadingButton.classList.add('gt-hidden');
  addHere.querySelector('.branch-tag-area-text').textContent = loadingButton.getAttribute('data-contained-in-text');
  addTags(data.tags, addHere.querySelector('.tag-area'));
  const branchArea = addHere.querySelector('.branch-area');
  addBranches(data.branches, data.default_branch, branchArea.getAttribute('data-defaultbranch-tooltip'), branchArea);
}

function addTags(tags, addHere) {
  if (tags.length > 0) showAreas('.tag-area,.tag-area-parent');
  for (const tag of tags) {
    addLink(tag.web_url, tag.name, addHere);
  }
}

function addBranches(branches, defaultBranch, defaultBranchTooltip, addHere) {
  if (branches.length > 0) showAreas('.branch-area,.branch-area-parent');
  for (const branch of branches) {
    addLink(branch.web_url, branch.name, addHere, defaultBranch === branch.name ? defaultBranchTooltip : undefined);
  }
}

function showAreas(selector) {
  for (const branchArea of document.querySelectorAll(selector)) branchArea.classList.remove('gt-hidden');
}

function addLink(href, text, addHere, tooltip) {
  const link = document.createElement('a');
  link.classList.add('muted', 'gt-px-3', 'gt-rounded');
  if (tooltip) {
    link.classList.add('gt-border-secondary');
    link.setAttribute('data-tooltip-content', tooltip);
  }
  link.href = href;
  link.textContent = text;
  addHere.append(link);
}

export function initLoadBranchesAndTagsButton() {
  for (const loadButton of document.querySelectorAll('.load-tags-and-branches')) loadButton.addEventListener('click', async () => loadBranchesAndTags(loadButton, document.querySelector('.branch-and-tag-area')));
}
