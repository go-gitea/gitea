import {GET} from '../modules/fetch.ts';
import {matchEmoji, matchMention} from './match.ts';

vi.mock('../modules/fetch.ts', () => ({
  GET: vi.fn(),
}));

const testMentions = [
  {key: 'user1 User 1', value: 'user1', name: 'user1', fullname: 'User 1', avatar: 'https://avatar1.com'},
  {key: 'user2 User 2', value: 'user2', name: 'user2', fullname: 'User 2', avatar: 'https://avatar2.com'},
  {key: 'org3 User 3', value: 'org3', name: 'org3', fullname: 'User 3', avatar: 'https://avatar3.com'},
  {key: 'user4 User 4', value: 'user4', name: 'user4', fullname: 'User 4', avatar: 'https://avatar4.com'},
  {key: 'user5 User 5', value: 'user5', name: 'user5', fullname: 'User 5', avatar: 'https://avatar5.com'},
  {key: 'org6 User 6', value: 'org6', name: 'org6', fullname: 'User 6', avatar: 'https://avatar6.com'},
  {key: 'org7 User 7', value: 'org7', name: 'org7', fullname: 'User 7', avatar: 'https://avatar7.com'},
];

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

test('matchMention', async () => {
  const oldLocation = String(window.location);
  window.location.assign('http://localhost/owner/repo/issues/1');
  vi.mocked(GET).mockResolvedValue({ok: true, json: () => Promise.resolve(testMentions)} as Response);
  expect(await matchMention('')).toEqual(testMentions.slice(0, 6));
  expect(await matchMention('user4')).toEqual([testMentions[3]]);
  window.location.assign(oldLocation);
});
