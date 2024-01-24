import {convertName} from './init.js';

test('init', () => {
  expect(convertName('abc')).toEqual('abc');
  expect(convertName('abc-repo')).toEqual('abcRepo');
  expect(convertName('abc-repo-issue')).toEqual('abcRepoIssue');
});
