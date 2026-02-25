import emojis from '../../../assets/emoji.json' with {type: 'json'};
import {GET} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {parseIssueHref, parseRepoOwnerPathInfo} from '../utils.ts';
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

let cachedMentions: Mention[];

export async function fetchMentions(): Promise<Mention[]> {
  if (!cachedMentions) {
    cachedMentions = [];
    const {ownerName, repoName} = parseRepoOwnerPathInfo(window.location.pathname);
    if (ownerName && repoName) {
      try {
        const {indexString} = parseIssueHref(window.location.href);
        const query = indexString ? `?issue_index=${indexString}` : '';
        const res = await GET(`${window.config.appSubUrl}/${ownerName}/${repoName}/-/mentions${query}`);
        cachedMentions = await res.json();
      } catch (e) {
        showErrorToast(`Failed to load mentions: ${e}`);
      }
    }
  }
  return cachedMentions;
}

type MentionSuggestion = {value: string; name: string; fullname: string; avatar: string};
export async function matchMention(queryText: string): Promise<MentionSuggestion[]> {
  const values = await fetchMentions();
  const query = queryText.toLowerCase();

  // results is a map of weights, lower is better
  const results = new Map<MentionSuggestion, number>();
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
