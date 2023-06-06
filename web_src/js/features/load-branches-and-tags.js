const {csrfToken} = window.config;

function loadBranchesAndTags(loadingButton, addHere) {
  loadingButton.setAttribute('disabled', 'disabled');
  const response = await fetch(loadingButton.getAttribute('data-fetch-url'), {
    method: 'GET',
    headers: {'X-Csrf-Token': csrfToken},
    body: JSON.stringify(data),
  }).finally(() => loadingButton.removeAttribute('disabled'));

  const data  = await response.json();
  addTags(data.tags, addHere);
  addBranches(data.branches, addHere);
}

function addTags(tags, addHere) {
  for(const tag of tags)
    addLink(tag.link, addHere);
}

function addTags(tags, addHere) {
  for(const branch of branches)
    addLink(branch.link, branch.name, addHere);
}

function addLink(href, text, parent) {
  const link = document.createElement('a');
  link.href=href;
  link.text=text;
  parent.appendChild(link);
}

export function initLoadBranchesAndTagsButton() {
  for(const loadButton of document.querySelector('.load-tags-and-branches-button'))
    loadButton.addEventListener('click', (e) => loadBranchesAndTags(e.target, ));
}
