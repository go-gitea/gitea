import {toAbsoluteLocaleDate} from './absolute-date.ts';

test('toAbsoluteLocaleDate', () => {
  expect(toAbsoluteLocaleDate('2024-03-15', 'en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })).toEqual('March 15, 2024');

  expect(toAbsoluteLocaleDate('2024-03-15', 'de-DE', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })).toEqual('15. März 2024');
});
