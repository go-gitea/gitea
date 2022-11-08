import {test, expect} from 'vitest';
import {singleRe, rangeRe} from './repo-code.js';

test('singleRe', () => {
  expect(singleRe.test('#L0')).toEqual(false);
  expect(singleRe.test('#L1')).toEqual(true);
  expect(singleRe.test('#L01')).toEqual(false);
  expect(singleRe.test('#n0')).toEqual(false);
  expect(singleRe.test('#n1')).toEqual(true);
  expect(singleRe.test('#n01')).toEqual(false);
});

test('rangeRe', () => {
  expect(rangeRe.test('#L0-L10')).toEqual(false);
  expect(rangeRe.test('#L1-L10')).toEqual(true);
  expect(rangeRe.test('#L01-L10')).toEqual(false);
  expect(rangeRe.test('#L1-L01')).toEqual(false);
});
