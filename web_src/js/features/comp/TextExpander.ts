import {matchEmoji, matchMention, matchIssue} from '../../utils/match.ts';
import {emojiString} from '../emoji.ts';
import {svg} from '../../svg.ts';

type Issue = {state: 'open' | 'closed'; pull_request: {draft: boolean; merged: boolean} | null};
function getIssueIcon(issue: Issue) {
  if (issue.pull_request !== null) {
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
  if (issue.pull_request !== null) {
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
        const url = window.location.href;
        const matches = await matchIssue(url, text);
        if (!matches.length) return resolve({matched: false});

        const ul = document.createElement('ul');
        ul.classList.add('suggestions');
        for (const {value, name, issue} of matches) {
          const li = document.createElement('li');
          li.classList.add('tw-flex', 'tw-gap-2');
          li.setAttribute('role', 'option');
          li.setAttribute('data-value', `${key}${value}`);

          const icon = document.createElement('div');
          icon.innerHTML = svg(getIssueIcon(issue), 16, ['text', getIssueColor(issue)].join(' ')).trim();
          li.append(icon.firstChild);

          const id = document.createElement('span');
          id.classList.add('id');
          id.textContent = value;
          li.append(id);

          const nameSpan = document.createElement('span');
          nameSpan.textContent = name;
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
