import emojis from '../../../assets/emoji.json';

const {StaticUrlPrefix} = window.config;

const tempMap = {gitea: 'gitea'};
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

export const emojiMap = {};
for (const key of emojiKeys) {
  emojiMap[key] = tempMap[key];
}

export function emojiHTML(name) {
  let inner;
  if (name === 'gitea') {
    inner = `<img class="emoji" alt=":${name}:" src="${StaticUrlPrefix}/img/emoji.png" align="absmiddle">`;
  } else if (emojiMap[name]) {
    inner = emojiMap[name];
  } else {
    inner = `:${name}:`;
  }

  return `<span class="emoji" title=":${name}:">${inner}</span>`;
}
