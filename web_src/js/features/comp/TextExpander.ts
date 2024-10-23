import {matchEmoji, matchMention, matchIssue} from '../../utils/match.ts';
import {emojiString} from '../emoji.ts';
import {svg} from '../../svg.ts';
import {parseIssueHref} from '../../utils.ts';
import {createElementFromAttrs, createElementFromHTML} from '../../utils/dom.ts';

type Issue = {id: number; title: string; state: 'open' | 'closed'; pull_request?: {draft: boolean; merged: boolean}};
function getIssueIcon(issue: Issue) {
  if (issue.pull_request) {
    if (issue.state === 'open') {
      if (issue.pull_request.draft === true) {
        return 'octicon-git-pull-request-draft'; // WIP PR
      }
      return 'octicon-git-pull-request'; // Open PR
    } else if (issue.pull_request.merged === true) {
      return 'octicon-git-merge'; // Merged PR
    }
    return 'octicon-git-pull-request'; // Closed PR
  } else if (issue.state === 'open') {
    return 'octicon-issue-opened'; // Open Issue
  }
  return 'octicon-issue-closed'; // Closed Issue
}

function getIssueColor(issue: Issue) {
  if (issue.pull_request) {
    if (issue.pull_request.draft === true) {
      return 'grey'; // WIP PR
    } else if (issue.pull_request.merged === true) {
      return 'purple'; // Merged PR
    }
  }
  if (issue.state === 'open') {
    return 'green'; // Open Issue
  }
  return 'red'; // Closed Issue
}

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
      provide(new Promise(async (resolve) => {
        const {owner, repo, index} = parseIssueHref(window.location.href);
        const matches = await matchIssue(owner, repo, index, text);
        if (!matches.length) return resolve({matched: false});

        const ul = document.createElement('ul');
        ul.classList.add('suggestions');
        for (const issue of matches) {
          const li = createElementFromAttrs('li', {
            role: 'option',
            'data-value': `${key}${issue.id}`,
          });
          li.classList.add('tw-flex', 'tw-gap-2');

          const icon = svg(getIssueIcon(issue), 16, ['text', getIssueColor(issue)].join(' '));
          li.append(createElementFromHTML(icon));

          const id = document.createElement('span');
          id.classList.add('id');
          id.textContent = issue.id.toString();
          li.append(id);

          const nameSpan = document.createElement('span');
          nameSpan.textContent = issue.title;
          li.append(nameSpan);

          ul.append(li);
        }

        resolve({matched: true, fragment: ul});
      }));
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
