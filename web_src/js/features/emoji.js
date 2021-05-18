import emojis from '../../../assets/emoji.json';

const {AssetUrlPrefix} = window.config;

const tempMap = {gitea: ':gitea:'};
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

const emojiMap = {};
for (const key of emojiKeys) {
  emojiMap[key] = tempMap[key];
}

// retrieve HTML for given emoji name
export function emojiHTML(name) {
  let inner;
  if (name === 'gitea') {
    inner = `<img alt=":${name}:" src="${AssetUrlPrefix}/img/emoji/gitea.png">`;
  } else {
    inner = emojiString(name);
  }

  return `<span class="emoji" title=":${name}:">${inner}</span>`;
}

// retrieve string for given emoji name
export function emojiString(name) {
  return emojiMap[name] || `:${name}:`;
}
