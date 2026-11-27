import emojis from '../../../public/assets/emoji.json' with {type: 'json'};
import {GET} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {parseIssuePageInfo} from '../utils.ts';
import {errorMessage} from '../modules/errors.ts';
import type {Mention} from '../types.ts';

const maxMatches = 6;

function sortAndReduce<T>(map: Map<T, number>): T[] {
  const sortedMap = new Map(Array.from(map).sort((a, b) => a[1] - b[1]));
  return Array.from(sortedMap.keys()).slice(0, maxMatches);
}

export function matchEmoji(queryText: string): string[] {
  const query = queryText.toLowerCase().replaceAll('_', ' ');
  if (!query) return emojis.slice(0, maxMatches).map((e) => e.aliases[0]);

  // results is a map of weights, lower is better
  const results = new Map<string, number>();
  for (const {aliases} of emojis) {
    let rank = Infinity;
    for (const [aliasIndex, alias] of aliases.entries()) {
      const normalizedAlias = alias.replaceAll('_', ' ');
      const index = normalizedAlias.indexOf(query);
      if (index !== -1) rank = Math.min(rank, normalizedAlias === query ? -1 : index + aliasIndex);
    }
    if (rank !== Infinity) results.set(aliases[0], rank);
  }

  return sortAndReduce(results);
}

let cachedMentionsPromise: Promise<Mention[]> | undefined;
let cachedMentionsUrl: string;

export function fetchMentions(mentionsUrl: string): Promise<Mention[]> {
  if (cachedMentionsPromise && cachedMentionsUrl === mentionsUrl) {
    return cachedMentionsPromise;
  }
  cachedMentionsUrl = mentionsUrl;
  cachedMentionsPromise = (async () => {
    try {
      const issueIndex = parseIssuePageInfo().issueNumber;
      const query = issueIndex ? `?issue_index=${issueIndex}` : '';
      const res = await GET(`${mentionsUrl}${query}`);
      if (!res.ok) throw new Error(res.statusText);
      return await res.json() as Mention[];
    } catch (e) {
      showErrorToast(`Failed to load mentions: ${errorMessage(e)}`);
      return [];
    }
  })();
  return cachedMentionsPromise;
}

export async function matchMention(mentionsUrl: string, queryText: string): Promise<Mention[]> {
  const values = await fetchMentions(mentionsUrl);
  const query = queryText.toLowerCase();

  // results is a map of weights, lower is better
  const results = new Map<Mention, number>();
  for (const obj of values) {
    const index = obj.key.toLowerCase().indexOf(query);
    if (index === -1) continue;
    const existing = results.get(obj);
    results.set(obj, existing ? existing - index : index);
  }

  return sortAndReduce(results);
}
