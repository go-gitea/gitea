import emojis from '../../../assets/emoji.json';

const {StaticUrlPrefix} = window.config;

export const giteaImage = `<img alt=":gitea:" title=":gitea:" class="emoji" src="${StaticUrlPrefix}/img/emoji.png" align="absmiddle">`;

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
