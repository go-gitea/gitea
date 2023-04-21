import {test, expect} from 'vitest';
import {matchEmoji, matchMention} from './match.js';

test('matchEmoji', () => {
  expect(matchEmoji('')).toEqual([
    '+1',
    '-1',
    '100',
    '1234',
    '1st_place_medal',
    '2nd_place_medal',
  ]);

  expect(matchEmoji('hear')).toEqual([
    'hear_no_evil',
    'heard_mcdonald_islands',
    'heart',
    'heart_decoration',
    'heart_eyes',
    'heart_eyes_cat',
  ]);
});

test('matchMention', () => {
  expect(matchMention('')).toEqual(window.config.tributeValues.slice(0, 6));
  expect(matchMention('user4')).toEqual([window.config.tributeValues[3]]);
});
