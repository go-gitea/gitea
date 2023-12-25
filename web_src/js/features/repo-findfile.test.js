import {strSubMatch, calcMatchedWeight, filterRepoFilesWeighted} from './repo-findfile.js';

describe('Repo Find Files', () => {
  test('strSubMatch', () => {
    expect(strSubMatch('abc', '')).toEqual(['abc']);
    expect(strSubMatch('abc', 'a')).toEqual(['', 'a', 'bc']);
    expect(strSubMatch('abc', 'b')).toEqual(['a', 'b', 'c']);
    expect(strSubMatch('abc', 'c')).toEqual(['ab', 'c']);
    expect(strSubMatch('abc', 'ac')).toEqual(['', 'a', 'b', 'c']);
    expect(strSubMatch('abc', 'z')).toEqual(['abc']);
    expect(strSubMatch('abc', 'az')).toEqual(['abc']);

    expect(strSubMatch('ABc', 'ac')).toEqual(['', 'A', 'B', 'c']);
    expect(strSubMatch('abC', 'ac')).toEqual(['', 'a', 'b', 'C']);

    expect(strSubMatch('aabbcc', 'abc')).toEqual(['', 'a', 'a', 'b', 'b', 'c', 'c']);
    expect(strSubMatch('the/directory', 'hedir')).toEqual(['t', 'he', '/', 'dir', 'ectory']);
  });

  test('calcMatchedWeight', () => {
    expect(calcMatchedWeight(['a', 'b', 'c', 'd']) < calcMatchedWeight(['a', 'bc', 'c'])).toBeTruthy();
  });

  test('filterRepoFilesWeighted', () => {
    // the first matched result should always be the "word.txt"
    let res = filterRepoFilesWeighted(['word.txt', 'we-got-result.dat'], 'word');
    expect(res).toHaveLength(2);
    expect(res[0].matchResult).toEqual(['', 'word', '.txt']);

    res = filterRepoFilesWeighted(['we-got-result.dat', 'word.txt'], 'word');
    expect(res).toHaveLength(2);
    expect(res[0].matchResult).toEqual(['', 'word', '.txt']);
  });
});
