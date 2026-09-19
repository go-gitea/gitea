import {gapExcerptUrl, gapExpandDirection, gapReachesFileEnd, type DiffGap} from './repo-diff-gaps.ts';

function gap(partial: Partial<DiffGap>): DiffGap {
  return {key: 'k', anchor: 'a', lastLeft: 0, lastRight: 0, left: 0, right: 0, leftHunk: 0, rightHunk: 0, hiddenCommentIds: [], ...partial};
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
  // a gap that fits in one chunk is revealed in one go
  expect(gapExpandDirection(gap({lastLeft: 10, lastRight: 10, left: 20, right: 20, leftHunk: 5, rightHunk: 5}))).toEqual('single');
});

test('gapReachesFileEnd', () => {
  expect(gapReachesFileEnd(gap({leftHunk: 0, rightHunk: 0}))).toBe(true);
  expect(gapReachesFileEnd(gap({leftHunk: 7, rightHunk: 7}))).toBe(false);
});

test('gapExcerptUrl carries the gap the backend needs', () => {
  const url = new URL(gapExcerptUrl('/user/repo/blob_excerpt/sha?style=split&path=a.txt', gap({key: '0-17', anchor: 'diff-abcK17', left: 17, right: 17, leftHunk: 7, rightHunk: 7}), 'up'));
  expect(url.pathname).toEqual('/user/repo/blob_excerpt/sha');
  expect(Object.fromEntries(url.searchParams)).toMatchObject({
    style: 'split', path: 'a.txt', direction: 'up', anchor: 'diff-abcK17', gap_key: '0-17',
    last_left: '0', last_right: '0', left: '17', right: '17', left_hunk_size: '7', right_hunk_size: '7',
  });
});
