import {applyMergeBoxRefresh} from './repo-issue-pull.ts';

test('applyMergeBoxRefresh replaces the element with the refreshed HTML', () => {
  document.body.innerHTML = '<div class="pull-merge-box">old</div><span>after</span>';
  const el = document.querySelector('.pull-merge-box')!;

  const replaced = applyMergeBoxRefresh(el, '<div class="pull-merge-box">new</div>');

  expect(replaced).toBe(true);
  expect(document.body.innerHTML).toEqual('<div class="pull-merge-box">new</div><span>after</span>');
});

test('applyMergeBoxRefresh keeps the existing element when the response body is empty', () => {
  document.body.innerHTML = '<div class="pull-merge-box">old</div><span>after</span>';
  const el = document.querySelector('.pull-merge-box')!;

  const replaced = applyMergeBoxRefresh(el, '');

  expect(replaced).toBe(false);
  expect(document.body.innerHTML).toEqual('<div class="pull-merge-box">old</div><span>after</span>');
});
