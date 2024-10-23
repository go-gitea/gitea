import emojis from '../../../assets/emoji.json';
import { request } from '../modules/fetch.ts';

const maxMatches = 6;

function sortAndReduce<T>(map: Map<T, number>): T[] {
  const sortedMap = new Map(Array.from(map.entries()).sort((a, b) => a[1] - b[1]));
  return Array.from(sortedMap.keys()).slice(0, maxMatches);
}

export function matchEmoji(queryText: string): string[] {
  const query = queryText.toLowerCase().replaceAll('_', ' ');
  if (!query) return emojis.slice(0, maxMatches).map((e) => e.aliases[0]);

  // results is a map of weights, lower is better
  const results = new Map<string, number>();
  for (const {aliases} of emojis) {
    const mainAlias = aliases[0];
    for (const [aliasIndex, alias] of aliases.entries()) {
      const index = alias.replaceAll('_', ' ').indexOf(query);
      if (index === -1) continue;
      const existing = results.get(mainAlias);
      const rankedIndex = index + aliasIndex;
      results.set(mainAlias, existing ? existing - rankedIndex : rankedIndex);
    }
  }

  return sortAndReduce(results);
}

type Mention = {value: string; name: string; fullname: string; avatar: string};
export function matchMention(queryText: string): Mention[] {
  const query = queryText.toLowerCase();

  // results is a map of weights, lower is better
  const results = new Map<Mention, number>();
  for (const obj of window.config.mentionValues ?? []) {
    const index = obj.key.toLowerCase().indexOf(query);
    if (index === -1) continue;
    const existing = results.get(obj);
    results.set(obj, existing ? existing - index : index);
  }

  return sortAndReduce(results);
}

type Issue = {state: 'open' | 'closed'; pull_request: {draft: boolean; merged: boolean} | null};
type IssueMention = {value: string; name: string; issue: Issue};
export async function matchIssue(url: string, queryText: string): Promise<IssueMention[]> {
  const query = queryText.toLowerCase();

  // http://localhost:3000/anbraten/test/issues/1
  // http://localhost:3000/anbraten/test/compare/main...anbraten-patch-1
  const repository = (new URL(url)).pathname.split('/').slice(1, 3).join('/');
  const issuePullRequestId = url.split('/').slice(-1)[0];

  console.log('suggestions for 1', {
    repository,
    query,
  });

  // TODO: fetch data from api
  // const res = await request('/-/suggestions', {
  //   method: 'GET',
  //   data: {
  //     repository,
  //     query,
  //   },
  // });
  // console.log(await res.json());

  // results is a map of weights, lower is better
  const results = new Map<IssueMention, number>();
  // for (const obj of window.config.mentionValues ?? []) {
  //   const index = obj.key.toLowerCase().indexOf(query);
  //   if (index === -1) continue;
  //   const existing = results.get(obj);
  //   results.set(obj, existing ? existing - index : index);
  // }

  results.set({
    value: '28958',
    name: 'Live removal of issue comments using htmx websocket',
    issue: {
      state: 'open',
      pull_request: {
        merged: false,
        draft: false,
      },
    },
  }, 0);

  results.set({
    value: '32234',
    name: 'Calculate `PublicOnly` for org membership only once',
    issue: {
      state: 'closed',
      pull_request: {
        merged: true,
        draft: false,
      },
    },
  }, 1);

  results.set({
    value: '32280',
    name: 'Optimize branch protection rule loading',
    issue: {
      state: 'open',
      pull_request: {
        merged: false,
        draft: false,
      },
    },
  }, 2);

  results.set({
    value: '32326',
    name: 'Shallow Mirroring',
    issue: {
      state: 'open',
      pull_request: null,
    },
  }, 3);

  results.set({
    value: '32248',
    name: 'Make admins adhere to branch protection rules',
    issue: {
      state: 'closed',
      pull_request: {
        merged: true,
        draft: false,
      },
    },
  }, 4);

  results.set({
    value: '32249',
    name: 'Add a way to disable branch protection rules for admins',
    issue: {
      state: 'closed',
      pull_request: null,
    },
  }, 5);

  // filter out current issue/pull request
  for (const [key] of results.entries()) {
    if (key.value === issuePullRequestId) {
      results.delete(key);
      break;
    }
  }

  return sortAndReduce(results);
}
