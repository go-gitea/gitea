import {getPaginationPages, type PaginationPage} from './pagination.ts';

// The expected pages are what modules/paginator.Paginator.Pages renders.
const cases: Array<[current: number, total: number, pages: PaginationPage[]]> = [
  // all pages fit in one row
  [1, 0, []],
  [1, 1, [1]],
  [1, 5, [1, 2, 3, 4, 5]],
  [5, 5, [1, 2, 3, 4, 5]],
  // five numbers around the current page, three of them near either end
  [1, 20, [1, 2, 3, 4, 5, null]],
  [3, 20, [1, 2, 3, 4, 5, null]],
  [4, 20, [null, 2, 3, 4, 5, 6, null]],
  [10, 20, [null, 8, 9, 10, 11, 12, null]],
  [18, 20, [null, 16, 17, 18, 19, 20]],
  [20, 20, [null, 16, 17, 18, 19, 20]],
  // the first page past a single row
  [1, 6, [1, 2, 3, 4, 5, null]],
  [4, 6, [null, 2, 3, 4, 5, 6]],
  [4, 7, [null, 2, 3, 4, 5, 6, null]],
  [5, 7, [null, 3, 4, 5, 6, 7]],
  // a page from the address bar past the end is clamped like the server does
  [99, 20, [null, 16, 17, 18, 19, 20]],
];

test('getPaginationPages', () => {
  for (const [current, total, pages] of cases) {
    expect(getPaginationPages(current, total)).toEqual(pages);
  }
});
