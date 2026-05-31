import {filterDependencySearchResponse} from './repo-issue-sidebar.ts';

test('filterDependencySearchResponse skips current issue and existing dependencies', () => {
  const response = [
    {id: 1, number: 1, title: 'current issue', repository: {full_name: 'owner/repo'}},
    {id: 2, number: 2, title: 'new dependency', repository: {full_name: 'owner/repo'}},
    {id: 3, number: 3, title: 'existing dependency', repository: {full_name: 'owner/repo'}},
  ];

  const result = filterDependencySearchResponse(response, '1', new Set(['3']));

  expect(result.success).toBe(true);
  expect(result.results.map((item) => item.value)).toEqual([2]);
});
