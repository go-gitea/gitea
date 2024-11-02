import {toAbsoluteLocaleDate} from './absolute-date.ts';

test('toAbsoluteLocaleDate', () => {
  expect(toAbsoluteLocaleDate('2024-03-15', 'en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })).toEqual('March 15, 2024');

  expect(toAbsoluteLocaleDate('2024-03-15T01:02:03', 'de-DE', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })).toEqual('15. März 2024');

  // these cases shouldn't happen
  expect(toAbsoluteLocaleDate('2024-03-15 01:02:03', '', {})).toEqual('Invalid Date');
  expect(toAbsoluteLocaleDate('10000-01-01', '', {})).toEqual('Invalid Date');

  // test different timezone
  const oldTZ = process.env.TZ;
  process.env.TZ = 'America/New_York';
  expect(new Date('2024-03-15').toLocaleString()).toEqual('3/14/2024, 8:00:00 PM');
  expect(toAbsoluteLocaleDate('2024-03-15')).toEqual('3/15/2024, 12:00:00 AM');
  process.env.TZ = oldTZ;
});
