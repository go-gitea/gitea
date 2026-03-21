import {onInputDebounce, toggleElem} from '../utils/dom.ts';
import {GET} from '../modules/fetch.ts';

const {appSubUrl} = window.config;
const reIssueIndex = /^(\d+)$/; // eg: "123"
const reIssueSharpIndex = /^#(\d+)$/; // eg: "#123"
const reIssueOwnerRepoIndex = /^([-.\w]+)\/([-.\w]+)#(\d+)$/;  // eg: "{owner}/{repo}#{index}"

// if the searchText can be parsed to an "issue goto link", return the link, otherwise return empty string
export function parseIssueListQuickGotoLink(repoLink: string, searchText: string) {
  searchText = searchText.trim();
  let targetUrl = '';
  if (repoLink) {
    // try to parse it in current repo
    if (reIssueIndex.test(searchText)) {
      targetUrl = `${repoLink}/issues/${searchText}`;
    } else if (reIssueSharpIndex.test(searchText)) {
      targetUrl = `${repoLink}/issues/${searchText.substring(1)}`;
    }
  }
  // try to parse it for a global search (eg: "owner/repo#123")
  const [_, owner, repo, index] = reIssueOwnerRepoIndex.exec(searchText) || [];
  if (owner) {
    targetUrl = `${appSubUrl}/${owner}/${repo}/issues/${index}`;
  }
  return targetUrl;
}

export function initCommonIssueListQuickGoto() {
  const elGotoButton = document.querySelector<HTMLElement>('#issue-list-quick-goto');
  if (!elGotoButton) return;

  const form = elGotoButton.closest('form')!;
  const input = form.querySelector<HTMLInputElement>('input[name=q]')!;
  const repoLink = elGotoButton.getAttribute('data-repo-link') || '';

  elGotoButton.addEventListener('click', () => {
    window.location.href = elGotoButton.getAttribute('data-issue-goto-link')!;
  });

  const onInput = async () => {
    const searchText = input.value;
    // try to check whether the parsed goto link is valid
    let targetUrl = parseIssueListQuickGotoLink(repoLink, searchText);
    if (targetUrl) {
      const res = await GET(`${targetUrl}/info`); // backend: GetIssueInfo, it only checks whether the issue exists by status code
      if (res.status !== 200) targetUrl = '';
    }
    // if the input value has changed, then ignore the result
    if (input.value !== searchText) return;

    toggleElem(elGotoButton, Boolean(targetUrl));
    elGotoButton.setAttribute('data-issue-goto-link', targetUrl);
  };

  input.addEventListener('input', onInputDebounce(onInput));
  onInput();
}
