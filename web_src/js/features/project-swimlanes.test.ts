import {moveWithinColumn} from './project-swimlanes.ts';

test('moving a card preserves the order of cards in other label lanes', () => {
  const column = [10, 20, 30, 40];
  expect(moveWithinColumn(column, [50, 20, 40], 50)).toEqual([10, 50, 20, 30, 40]);
  expect(moveWithinColumn(column, [40, 20], 40)).toEqual([10, 40, 20, 30]);
  expect(moveWithinColumn(column, [20, 50, 40], 50)).toEqual([10, 20, 30, 50, 40]);
  expect(moveWithinColumn(column, [20, 40, 50], 50)).toEqual([10, 20, 30, 40, 50]);
  expect(moveWithinColumn(column, [50], 50)).toEqual([10, 20, 30, 40, 50]);
  expect(moveWithinColumn([], [50], 50)).toEqual([50]);
  expect(column).toEqual([10, 20, 30, 40]);
});
