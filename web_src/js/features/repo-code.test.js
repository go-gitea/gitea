import {singleAnchorRegex, rangeAnchorRegex} from './repo-code.js';

test('singleAnchorRegex', () => {
  expect(singleAnchorRegex.test('#L0')).toEqual(false);
  expect(singleAnchorRegex.test('#L1')).toEqual(true);
  expect(singleAnchorRegex.test('#L01')).toEqual(false);
  expect(singleAnchorRegex.test('#n0')).toEqual(false);
  expect(singleAnchorRegex.test('#n1')).toEqual(true);
  expect(singleAnchorRegex.test('#n01')).toEqual(false);
});

test('rangeAnchorRegex', () => {
  expect(rangeAnchorRegex.test('#L0-L10')).toEqual(false);
  expect(rangeAnchorRegex.test('#L1-L10')).toEqual(true);
  expect(rangeAnchorRegex.test('#L01-L10')).toEqual(false);
  expect(rangeAnchorRegex.test('#L1-L01')).toEqual(false);
});
