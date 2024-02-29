import {GET, POST, PATCH, PUT, DELETE} from './fetch.js';

// tests here are only to satisfy the linter for unused functions
test('exports', () => {
  expect(GET).toBeTruthy();
  expect(POST).toBeTruthy();
  expect(PATCH).toBeTruthy();
  expect(PUT).toBeTruthy();
  expect(DELETE).toBeTruthy();
});
