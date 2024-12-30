import {matchEmoji, matchMention, matchIssue} from '../../utils/match.ts';
import {emojiString} from '../emoji.ts';
import {svg} from '../../svg.ts';
import {parseIssueHref, parseIssueNewHref} from '../../utils.ts';
import {createElementFromAttrs, createElementFromHTML} from '../../utils/dom.ts';
import {getIssueColor, getIssueIcon} from '../issue.ts';
import {debounce} from 'perfect-debounce';

const debouncedSuggestIssues = debounce((key: string, text: string) => new Promise<{matched:boolean; fragment?: HTMLElement}>(async (resolve) => {
  let issuePathInfo = parseIssueHref(window.location.href);
  if (!issuePathInfo.ownerName) issuePathInfo = parseIssueNewHref(window.location.href);
  if (!issuePathInfo.ownerName) return resolve({matched: false});

  const matches = await matchIssue(issuePathInfo.ownerName, issuePathInfo.repoName, issuePathInfo.indexString, text);
  if (!matches.length) return resolve({matched: false});

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
  resolve({matched: true, fragment: ul});
}), 100);

export function initTextExpander(expander) {
  expander?.addEventListener('text-expander-change', ({detail: {key, provide, text}}) => {
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
      provide(debouncedSuggestIssues(key, text));
    }
  });
  expander?.addEventListener('text-expander-value', ({detail}) => {
    if (detail?.item) {
      // add a space after @mentions and #issue as it's likely the user wants one
      const suffix = ['@', '#'].includes(detail.key) ? ' ' : '';
      detail.value = `${detail.item.getAttribute('data-value')}${suffix}`;
    }
  });
}
