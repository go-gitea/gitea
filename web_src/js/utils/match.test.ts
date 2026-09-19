import {GET} from '../modules/fetch.ts';
import {matchMention} from './match.ts';

vi.mock('../modules/fetch.ts', () => ({GET: vi.fn()}));

const testMentions = [
  {key: 'user1 User 1', value: 'user1', name: 'user1', fullname: 'User 1', avatar: 'https://avatar1.com'},
  {key: 'user2 User 2', value: 'user2', name: 'user2', fullname: 'User 2', avatar: 'https://avatar2.com'},
  {key: 'org3 User 3', value: 'org3', name: 'org3', fullname: 'User 3', avatar: 'https://avatar3.com'},
  {key: 'user4 User 4', value: 'user4', name: 'user4', fullname: 'User 4', avatar: 'https://avatar4.com'},
  {key: 'user5 User 5', value: 'user5', name: 'user5', fullname: 'User 5', avatar: 'https://avatar5.com'},
  {key: 'org6 User 6', value: 'org6', name: 'org6', fullname: 'User 6', avatar: 'https://avatar6.com'},
  {key: 'org7 User 7', value: 'org7', name: 'org7', fullname: 'User 7', avatar: 'https://avatar7.com'},
];

test('matchMention', async () => {
  vi.mocked(GET).mockResolvedValue({ok: true, json: () => Promise.resolve(testMentions)} as Response);
  expect(await matchMention('/any-mentions', '')).toEqual(testMentions.slice(0, 6));
  expect(await matchMention('/any-mentions', 'user4')).toEqual([testMentions[3]]);
});
