import emojis from '../../../assets/emoji.json' with {type: 'json'};
import {GET} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {parseIssuePageInfo} from '../utils.ts';
import {errorMessage} from '../modules/errors.ts';
import type {Issue, Mention} from '../types.ts';

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

export async function matchIssue(owner: string, repo: string, issueIndexStr: string, query: string): Promise<Issue[]> {
  const res = await GET(`${window.config.appSubUrl}/${owner}/${repo}/issues/suggestions?q=${encodeURIComponent(query)}`);

  const issues: Issue[] = await res.json();
  const issueNumber = parseInt(issueIndexStr);

  // filter out issue with same id
  return issues.filter((i) => i.number !== issueNumber);
}
