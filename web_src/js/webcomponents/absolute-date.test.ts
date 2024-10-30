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

  expect(toAbsoluteLocaleDate('12345-03-15 01:02:03', '', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })).toEqual('Mar 15, 12345');
});
