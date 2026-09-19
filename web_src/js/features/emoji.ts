import emojis from '../../../assets/emoji.json' with {type: 'json'};
import {html} from '../utils/html.ts';
import {maxMatches, sortAndReduce} from '../utils/match.ts';

const {assetUrlPrefix, customEmojis} = window.config;
const emojiAliases = Object.values(emojis);

const tempMap = {...customEmojis};
for (const [emoji, aliases] of Object.entries(emojis)) {
  for (const alias of aliases) {
    tempMap[alias] = emoji;
  }
}

export const emojiKeys = Object.keys(tempMap).sort((a, b) => {
  if (a === '+1' || a === '-1') return -1;
  if (b === '+1' || b === '-1') return 1;
  return a.localeCompare(b);
});

const emojiMap: Record<string, string> = {};
for (const key of emojiKeys) {
  emojiMap[key] = tempMap[key];
}

// retrieve HTML for given emoji name
export function emojiHTML(name: string) {
  let inner;
  if (Object.hasOwn(customEmojis, name)) {
    inner = html`<img alt=":${name}:" src="${assetUrlPrefix}/img/emoji/${name}.png">`;
  } else {
    inner = emojiString(name);
  }
  return html`<span class="emoji" title=":${name}:">${inner}</span>`;
}

// retrieve string for given emoji name
export function emojiString(name: string) {
  return emojiMap[name] || `:${name}:`;
}

export function matchEmoji(queryText: string): string[] {
  const query = queryText.toLowerCase().replaceAll('_', ' ');
  if (!query) return emojiAliases.slice(0, maxMatches).map((aliases) => aliases[0]);

  // results is a map of weights, lower is better
  const results = new Map<string, number>();
  for (const aliases of emojiAliases) {
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
