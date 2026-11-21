import {initRepoIssueTimelineOptions} from './repo-issue.ts';
import {localUserSettings} from '../modules/user-settings.ts';

test('initRepoIssueTimelineOptions', () => {
  localUserSettings.setBoolean('issue-timeline-comments-only', false);
  document.body.innerHTML = `
    <a id="issue-timeline-comments-only"><svg class="tw-invisible"></svg></a>
    <div class="repository view issue"><div class="comment-list"></div></div>
  `;
  initRepoIssueTimelineOptions();
  const item = document.querySelector<HTMLElement>('#issue-timeline-comments-only')!;
  const list = document.querySelector('.comment-list')!;
  const check = item.querySelector('svg')!;

  item.click();
  expect(list.classList.contains('comments-only')).toBe(true);
  expect(check.classList.contains('tw-invisible')).toBe(false);
  expect(localUserSettings.getBoolean('issue-timeline-comments-only')).toBe(true);

  item.click();
  expect(list.classList.contains('comments-only')).toBe(false);
  expect(check.classList.contains('tw-invisible')).toBe(true);
  expect(localUserSettings.getBoolean('issue-timeline-comments-only')).toBe(false);
});
