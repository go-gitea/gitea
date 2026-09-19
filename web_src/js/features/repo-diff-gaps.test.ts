import {excerptChunkUrl, excerptGapsUrl, gapExpandDirection, gapReachesFileEnd, parseTableRows, type DiffGap} from './repo-diff-gaps.ts';

function gap(partial: Partial<DiffGap>): DiffGap {
  return {lastLeft: 0, lastRight: 0, left: 0, right: 0, leftHunk: 0, rightHunk: 0, hiddenCommentIds: [], ...partial};
}

// these mirror gitdiff.GetExpandDirection, which decided the arrows before the frontend did
test('gapExpandDirection', () => {
  // nothing is hidden between the two rendered lines
  expect(gapExpandDirection(gap({lastLeft: 5, lastRight: 5, left: 6, right: 6, leftHunk: 3, rightHunk: 3}))).toEqual('');
  // a gap at the top of the file only expands upwards
  expect(gapExpandDirection(gap({left: 17, right: 17, leftHunk: 7, rightHunk: 7}))).toEqual('up');
  // a gap wider than one chunk expands from either end
  expect(gapExpandDirection(gap({lastLeft: 23, lastRight: 23, left: 57, right: 57, leftHunk: 7, rightHunk: 7}))).toEqual('updown');
  // zero hunk sizes mean the gap runs to the end of the file
  expect(gapExpandDirection(gap({lastLeft: 103, lastRight: 103, left: 120, right: 120}))).toEqual('down');
  // a gap that fits in one chunk is expanded in one go
  expect(gapExpandDirection(gap({lastLeft: 10, lastRight: 10, left: 20, right: 20, leftHunk: 5, rightHunk: 5}))).toEqual('single');
});

test('gapReachesFileEnd', () => {
  expect(gapReachesFileEnd(gap({leftHunk: 0, rightHunk: 0}))).toBe(true);
  expect(gapReachesFileEnd(gap({leftHunk: 7, rightHunk: 7}))).toBe(false);
});

test('excerptChunkUrl names the gap and which end to expand from', () => {
  const url = new URL(excerptChunkUrl('/user/repo/blob_excerpt/sha?style=split&path=a.txt', gap({left: 17, right: 17, leftHunk: 7, rightHunk: 7}), 'up'));
  expect(url.pathname).toEqual('/user/repo/blob_excerpt/sha');
  expect(Object.fromEntries(url.searchParams)).toEqual({
    style: 'split', path: 'a.txt', gap: '0,0,17,17,7,7', direction: 'up',
  });
});

test('excerptGapsUrl names every gap and nothing else', () => {
  const gaps = [gap({lastLeft: 0, lastRight: 0, left: 17, right: 17, leftHunk: 7, rightHunk: 7}), gap({lastLeft: 23, lastRight: 23, left: 57, right: 57, leftHunk: 7, rightHunk: 7})];
  const url = new URL(excerptGapsUrl('/user/repo/blob_excerpt/sha?path=a.txt', gaps));
  expect(url.searchParams.getAll('gap')).toEqual(['0,0,17,17,7,7', '23,23,57,57,7,7']);
  expect(url.searchParams.has('direction')).toBe(false); // whole gaps, so no end to expand from
});

test('parseTableRows takes the rows out of a response that starts with a colgroup', () => {
  // the parser tucks rows after a colgroup into a tbody, which used to hide them from the caller
  const rows = parseTableRows('<colgroup><col width="50"></colgroup>\n<tr data-expand-gap="0-17"><td>a</td></tr>\n<tr data-expand-gap="0-17"><td>b</td></tr>');
  expect(rows.map((el) => el.tagName)).toEqual(['TR', 'TR']);
  expect(rows.map((el) => el.getAttribute('data-expand-gap'))).toEqual(['0-17', '0-17']);
});
