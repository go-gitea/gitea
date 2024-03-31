import emojis from '../../../assets/emoji.json';

const tempMap = {...window.config.customEmojis};
for (const {emoji, aliases} of emojis) {
  for (const alias of aliases || []) {
    tempMap[alias] = emoji;
  }
}

const emojiKeys = Object.keys(tempMap).sort((a, b) => {
  if (a === '+1' || a === '-1') return -1;
  if (b === '+1' || b === '-1') return 1;
  return a.localeCompare(b);
});

const emojiMap = {};
for (const key of emojiKeys) {
  emojiMap[key] = tempMap[key];
}

// retrieve string for given emoji name
export function emojiString(name) {
  return emojiMap[name] || `:${name}:`;
}
