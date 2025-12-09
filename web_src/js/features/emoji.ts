import emojis from '../../../assets/emoji.json' with {type: 'json'};
import {html} from '../utils/html.ts';

const {assetUrlPrefix, customEmojis} = window.config;

const tempMap = {...customEmojis};
for (const {emoji, aliases} of emojis) {
  for (const alias of aliases || []) {
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
