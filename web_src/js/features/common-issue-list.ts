import {isElemVisible, onInputDebounce, toggleElem} from '../utils/dom.ts';
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
  } else {
    // try to parse it for a global search (eg: "owner/repo#123")
    const [_, owner, repo, index] = reIssueOwnerRepoIndex.exec(searchText) || [];
    if (owner) {
      targetUrl = `${appSubUrl}/${owner}/${repo}/issues/${index}`;
    }
  }
  return targetUrl;
}

export function initCommonIssueListQuickGoto() {
  const goto = document.querySelector<HTMLElement>('#issue-list-quick-goto');
  if (!goto) return;

  const form = goto.closest('form')!;
  const input = form.querySelector<HTMLInputElement>('input[name=q]')!;
  const repoLink = goto.getAttribute('data-repo-link')!;
  let maxIssueIndex = 0;

  const redirectToIssuesLink = (link: string) => {
    if (link) window.location.href = link;
  };

  form.addEventListener('submit', (e) => {
    // if there is no goto button, or the form is submitted by non-quick-goto elements, submit the form directly
    if (!isElemVisible(goto) || reIssueIndex.test(input.value)) return;

    // if there is a goto button, use its link
    e.preventDefault();
    redirectToIssuesLink(goto.getAttribute('data-issue-goto-link') || '');
  });

  const onInput = async () => {
    const searchText = input.value;
    // try to check whether the parsed goto link is valid
    let targetUrl = (maxIssueIndex === 0 || Number(searchText) <= maxIssueIndex) && parseIssueListQuickGotoLink(repoLink, searchText) || '';
    if (targetUrl) {
      const res = await GET(`${targetUrl}/info`); // backend: GetIssueInfo, it only checks whether the issue exists by status code
      if (res.status !== 200) {
        targetUrl = '';
        const data = await res.json();
        maxIssueIndex = data.maxIndex;
      }
    }
    // if the input value has changed, then ignore the result
    if (input.value !== searchText) return;

    toggleElem(goto, Boolean(targetUrl));
    goto.setAttribute('data-issue-goto-link', targetUrl);
  };

  input.addEventListener('input', onInputDebounce(onInput));
  goto.addEventListener('click', () => redirectToIssuesLink(goto.getAttribute('data-issue-goto-link') || ''), {once: true});
}
