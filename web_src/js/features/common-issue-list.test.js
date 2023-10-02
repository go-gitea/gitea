import {parseIssueListQuickGotoLink} from './common-issue-list.js';

test('parseIssueListQuickGotoLink', () => {
  expect(parseIssueListQuickGotoLink('/link', '')).toEqual('');
  expect(parseIssueListQuickGotoLink('/link', 'abc')).toEqual('');
  expect(parseIssueListQuickGotoLink('/link', '123')).toEqual('/link/issues/123');
  expect(parseIssueListQuickGotoLink('/link', '#123')).toEqual('/link/issues/123');
  expect(parseIssueListQuickGotoLink('/link', 'owner/repo#123')).toEqual('');

  expect(parseIssueListQuickGotoLink('', '')).toEqual('');
  expect(parseIssueListQuickGotoLink('', 'abc')).toEqual('');
  expect(parseIssueListQuickGotoLink('', '123')).toEqual('');
  expect(parseIssueListQuickGotoLink('', '#123')).toEqual('');
  expect(parseIssueListQuickGotoLink('', 'owner/repo#')).toEqual('');
  expect(parseIssueListQuickGotoLink('', 'owner/repo#123')).toEqual('/owner/repo/issues/123');
});
