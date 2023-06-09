const {csrfToken} = window.config;

async function loadBranchesAndTags(loadingButton, addHere) {
  loadingButton.setAttribute('disabled', 'disabled');
  const response = await fetch(loadingButton.getAttribute('data-fetch-url'), {
    method: 'GET',
    headers: {'X-Csrf-Token': csrfToken},
  }).finally(() => loadingButton.removeAttribute('disabled'));

  const data  = await response.json();
  addTags(data.tags, addHere);
  addBranches(data.branches, addHere);
}

function addTags(tags, addHere) {
  for(const tag of tags)
    addLink(tag.web_url, addHere);
}

function addBranches(tags, addHere) {
  for(const branch of branches)
    addLink(branch.web_url, branch.name, addHere);
}

function addLink(href, text, parent) {
  const link = document.createElement('a');
  link.href=href;
  link.text=text;
  parent.appendChild(link);
}

export function initLoadBranchesAndTagsButton() {
  for(const loadButton of document.querySelectorAll('.load-tags-and-branches'))
    loadButton.addEventListener('click', async (e) => loadBranchesAndTags(loadButton, document.querySelector('.branch-and-tag-area')));
}
