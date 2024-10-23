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

type MentionSuggestion = {value: string; name: string; fullname: string; avatar: string};
export function matchMention(queryText: string): MentionSuggestion[] {
  const query = queryText.toLowerCase();

  // results is a map of weights, lower is better
  const results = new Map<MentionSuggestion, number>();
  for (const obj of window.config.mentionValues ?? []) {
    const index = obj.key.toLowerCase().indexOf(query);
    if (index === -1) continue;
    const existing = results.get(obj);
    results.set(obj, existing ? existing - index : index);
  }

  return sortAndReduce(results);
}

type Issue = {id: number; title: string; state: 'open' | 'closed'; pull_request?: {draft: boolean; merged: boolean}};
export async function matchIssue(url: string, queryText: string): Promise<Issue[]> {
  const query = queryText.toLowerCase();

  // TODO: support sub-path
  const repository = (new URL(url)).pathname.split('/').slice(1, 3).join('/');
  const issuePullRequestId = parseInt(url.split('/').slice(-1)[0]);

  const res = await request(`/api/v1/repos/${repository}/issues?q=${query}`, {
    method: 'GET',
  });

  const issues: Issue[] = await res.json();

  // filter issue with same id
  return issues.filter((i) => i.id !== issuePullRequestId);
}
