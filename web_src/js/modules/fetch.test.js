import {test, expect} from 'vitest';
import {GET, POST, PATCH, PUT, DELETE, isNetworkError} from './fetch.js';

test('exports', () => {
  expect(GET).toBeTruthy();
  expect(POST).toBeTruthy();
  expect(PATCH).toBeTruthy();
  expect(PUT).toBeTruthy();
  expect(DELETE).toBeTruthy();
});

test('isNetworkError', () => {
  expect(isNetworkError('Failed to fetch')).toEqual(true);
});
