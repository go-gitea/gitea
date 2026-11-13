import {matchEmoji, matchMention} from '../../utils/match.ts';
import {emojiString} from '../emoji.ts';
import {svg} from '../../svg.ts';
import {parseIssueHref, parseRepoOwnerPathInfo} from '../../utils.ts';
import {createElementFromAttrs, createElementFromHTML} from '../../utils/dom.ts';
import {getIssueColorClass, getIssueIcon} from '../issue.ts';
import {debounce} from '../../utils/func.ts';
import type TextExpanderElement from '@github/text-expander-element';
import type {TextExpanderChangeEvent, TextExpanderResult} from '@github/text-expander-element';
import {ExclusiveFetch} from '../../modules/fetch.ts';
import type {Issue} from '../../types.ts';

function makeFetchIssueSuggestionsUrl(query: string): {issueIndexStr: string, suggestionsUrl: string} | null {
  const hrefPathInfo = parseIssueHref(window.location.href);
  // the fallback has no indexString, it is only used to exclude the current issue when "matchIssue"
  const pathInfo = hrefPathInfo ?? parseRepoOwnerPathInfo(window.location.pathname);
  if (!pathInfo) return null;
  return {
    issueIndexStr: hrefPathInfo?.indexString ?? '',
    suggestionsUrl: `${window.config.appSubUrl}/${pathInfo.ownerName}/${pathInfo.repoName}/issues/suggestions?q=${encodeURIComponent(query)}`,
  };
}

function buildIssueSuggestions(key: string, matches: Issue[]): TextExpanderResult {
  if (matches.length === 0) return {matched: false};
  const ul = createElementFromAttrs('ul', {class: 'suggestions'});
  for (const issue of matches) {
    const li = createElementFromAttrs(
      'li', {role: 'option', class: 'tw-flex tw-gap-2', 'data-value': `${key}${issue.number}`},
      createElementFromHTML(svg(getIssueIcon(issue), 16, [getIssueColorClass(issue)])),
      createElementFromAttrs('span', null, `#${issue.number}`),
      createElementFromAttrs('span', null, issue.title),
    );
    ul.append(li);
  }
  return {matched: true, fragment: ul};
}

export function initTextExpander(expander: TextExpanderElement) {
  if (!expander) return;

  const textarea = expander.querySelector<HTMLTextAreaElement>('textarea')!;
  const mentionsUrl = expander.closest('[data-mentions-url]')?.getAttribute('data-mentions-url');

  // help to fix the text-expander "multiword+promise" bug: do not show the popup when there is no "#" before current line
  const shouldShowIssueSuggestions = () => {
    const posVal = textarea.value.substring(0, textarea.selectionStart);
    const lineStart = posVal.lastIndexOf('\n');
    const keyStart = posVal.lastIndexOf('#');
    return keyStart > lineStart;
  };

  const exclusiveFetch = new ExclusiveFetch();
  const debouncedIssueSuggestions = debounce(async (key: string, text: string): Promise<TextExpanderResult> => {
    // check the input before the request, to avoid emitting empty query to backend
    if (!shouldShowIssueSuggestions()) return {matched: false};

    const result = makeFetchIssueSuggestionsUrl(text);
    if (!result) return {matched: false};
    const {issueIndexStr, suggestionsUrl} = result;

    const resp = exclusiveFetch.request(suggestionsUrl);
    return await resp.fetchJson((raw: Issue[]) => {
      if (!shouldShowIssueSuggestions()) return {matched: false};

      const issueNumber = parseInt(issueIndexStr || '');
      const issues = raw.filter((i) => i.number !== issueNumber);
      return buildIssueSuggestions(key, issues);
    }) || {matched: false};
  }, 300); // to match onInputDebounce delay

  expander.addEventListener('text-expander-change', (e: TextExpanderChangeEvent) => {
    if (!e.detail) return;
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
      provide((async (): Promise<TextExpanderResult> => {
        if (!mentionsUrl) return {matched: false};
        const matches = await matchMention(mentionsUrl, text);
        if (!matches.length) return {matched: false};

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
          nameSpan.classList.add('name');
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

        return {matched: true, fragment: ul};
      })());
    } else if (key === '#') {
      provide(debouncedIssueSuggestions(key, text));
    }
  });

  expander.addEventListener('text-expander-value', (event) => {
    const {detail} = event as CustomEvent<{item: HTMLElement, key: string, value: string}>;
    if (detail?.item) {
      // add a space after @mentions and #issue as it's likely the user wants one
      const suffix = ['@', '#'].includes(detail.key) ? ' ' : '';
      detail.value = `${detail.item.getAttribute('data-value')}${suffix}`;
    }
  });
}
