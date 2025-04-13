import {matchEmoji, matchMention, matchIssue} from '../../utils/match.ts';
import {emojiString} from '../emoji.ts';
import {svg} from '../../svg.ts';
import {parseIssueHref, parseRepoOwnerPathInfo} from '../../utils.ts';
import {createElementFromAttrs, createElementFromHTML} from '../../utils/dom.ts';
import {getIssueColor, getIssueIcon} from '../issue.ts';
import {debounce} from 'perfect-debounce';
import type TextExpanderElement from '@github/text-expander-element';
import type {TextExpanderChangeEvent, TextExpanderResult} from '@github/text-expander-element';

async function fetchIssueSuggestions(key: string, text: string): Promise<TextExpanderResult> {
  const issuePathInfo = parseIssueHref(window.location.href);
  if (!issuePathInfo.ownerName) {
    const repoOwnerPathInfo = parseRepoOwnerPathInfo(window.location.pathname);
    issuePathInfo.ownerName = repoOwnerPathInfo.ownerName;
    issuePathInfo.repoName = repoOwnerPathInfo.repoName;
    // then no issuePathInfo.indexString here, it is only used to exclude the current issue when "matchIssue"
  }
  if (!issuePathInfo.ownerName) return {matched: false};

  const matches = await matchIssue(issuePathInfo.ownerName, issuePathInfo.repoName, issuePathInfo.indexString, text);
  if (!matches.length) return {matched: false};

  const ul = createElementFromAttrs('ul', {class: 'suggestions'});
  for (const issue of matches) {
    const li = createElementFromAttrs(
      'li', {role: 'option', class: 'tw-flex tw-gap-2', 'data-value': `${key}${issue.number}`},
      createElementFromHTML(svg(getIssueIcon(issue), 16, ['text', getIssueColor(issue)])),
      createElementFromAttrs('span', null, `#${issue.number}`),
      createElementFromAttrs('span', null, issue.title),
    );
    ul.append(li);
  }
  return {matched: true, fragment: ul};
}

export function initTextExpander(expander: TextExpanderElement) {
  if (!expander) return;

  const textarea = expander.querySelector<HTMLTextAreaElement>('textarea');

  // help to fix the text-expander "multiword+promise" bug: do not show the popup when there is no "#" before current line
  const shouldShowIssueSuggestions = () => {
    const posVal = textarea.value.substring(0, textarea.selectionStart);
    const lineStart = posVal.lastIndexOf('\n');
    const keyStart = posVal.lastIndexOf('#');
    return keyStart > lineStart;
  };

  const debouncedIssueSuggestions = debounce(async (key: string, text: string): Promise<TextExpanderResult> => {
    // https://github.com/github/text-expander-element/issues/71
    // Upstream bug: when using "multiword+promise", TextExpander will get wrong "key" position.
    // To reproduce, comment out the "shouldShowIssueSuggestions" check, use the "await sleep" below,
    // then use content "close #20\nclose #20\nclose #20" (3 lines), keep changing the last line `#20` part from the end (including removing the `#`)
    // There will be a JS error: Uncaught (in promise) IndexSizeError: Failed to execute 'setStart' on 'Range': The offset 28 is larger than the node's length (27).

    // check the input before the request, to avoid emitting empty query to backend (still related to the upstream bug)
    if (!shouldShowIssueSuggestions()) return {matched: false};
    // await sleep(Math.random() * 1000); // help to reproduce the text-expander bug
    const ret = await fetchIssueSuggestions(key, text);
    // check the input again to avoid text-expander using incorrect position (upstream bug)
    if (!shouldShowIssueSuggestions()) return {matched: false};
    return ret;
  }, 300); // to match onInputDebounce delay

  expander.addEventListener('text-expander-change', (e: TextExpanderChangeEvent) => {
    const {key, text, provide} = e.detail;
    if (key === ':') {
      const matches = matchEmoji(text);
      if (!matches.length) return provide({matched: false});

      const ul = document.createElement('ul');
      ul.classList.add('suggestions');
      for (const name of matches) {
        const emoji = emojiString(name);
        const li = document.createElement('li');
        li.setAttribute('role', 'option');
        li.setAttribute('data-value', emoji);
        li.textContent = `${emoji} ${name}`;
        ul.append(li);
      }

      provide({matched: true, fragment: ul});
    } else if (key === '@') {
      const matches = matchMention(text);
      if (!matches.length) return provide({matched: false});

      const ul = document.createElement('ul');
      ul.classList.add('suggestions');
      for (const {value, name, fullname, avatar} of matches) {
        const li = document.createElement('li');
        li.setAttribute('role', 'option');
        li.setAttribute('data-value', `${key}${value}`);

        const img = document.createElement('img');
        img.src = avatar;
        li.append(img);

        const nameSpan = document.createElement('span');
        nameSpan.textContent = name;
        li.append(nameSpan);

        if (fullname && fullname.toLowerCase() !== name) {
          const fullnameSpan = document.createElement('span');
          fullnameSpan.classList.add('fullname');
          fullnameSpan.textContent = fullname;
          li.append(fullnameSpan);
        }

        ul.append(li);
      }

      provide({matched: true, fragment: ul});
    } else if (key === '#') {
      provide(debouncedIssueSuggestions(key, text));
    }
  });

  expander.addEventListener('text-expander-value', ({detail}: Record<string, any>) => {
    if (detail?.item) {
      // add a space after @mentions and #issue as it's likely the user wants one
      const suffix = ['@', '#'].includes(detail.key) ? ' ' : '';
      detail.value = `${detail.item.getAttribute('data-value')}${suffix}`;
    }
  });
}
