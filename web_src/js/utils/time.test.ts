import {startDaysBetween} from './time.ts';

test('startDaysBetween', () => {
  expect(startDaysBetween(new Date('2024-02-15'), new Date('2024-04-18'))).toEqual([
    1708214400000,
    1708819200000,
    1709424000000,
    1710028800000,
    1710633600000,
    1711238400000,
    1711843200000,
    1712448000000,
    1713052800000,
  ]);
});
