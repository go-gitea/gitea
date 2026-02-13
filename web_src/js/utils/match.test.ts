import {matchEmoji, matchMention} from './match.ts';

test('matchEmoji', () => {
  expect(matchEmoji('')).toMatchInlineSnapshot(`
    [
      "+1",
      "-1",
      "100",
      "1234",
      "1st_place_medal",
      "2nd_place_medal",
    ]
  `);

  expect(matchEmoji('hea')).toMatchInlineSnapshot(`
    [
      "head_shaking_horizontally",
      "head_shaking_vertically",
      "headphones",
      "headstone",
      "health_worker",
      "hear_no_evil",
    ]
  `);

  expect(matchEmoji('hear')).toMatchInlineSnapshot(`
    [
      "hear_no_evil",
      "heard_mcdonald_islands",
      "heart",
      "heart_decoration",
      "heart_eyes",
      "heart_eyes_cat",
    ]
  `);

  expect(matchEmoji('poo')).toMatchInlineSnapshot(`
    [
      "poodle",
      "hankey",
      "spoon",
      "bowl_with_spoon",
    ]
  `);

  expect(matchEmoji('1st_')).toMatchInlineSnapshot(`
    [
      "1st_place_medal",
    ]
  `);

  expect(matchEmoji('jellyfis')).toMatchInlineSnapshot(`
    [
      "jellyfish",
    ]
  `);
});

test('matchMention', () => {
  expect(matchMention('')).toEqual(window.config.mentionValues.slice(0, 6));
  expect(matchMention('user4')).toEqual([window.config.mentionValues[3]]);
});
