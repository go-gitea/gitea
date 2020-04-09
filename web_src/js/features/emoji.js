import map from 'emojilib/simplemap.json';
import escapeStringRegexp from 'escape-string-regexp';
import findAndReplaceDOMText from 'findandreplacedomtext';

const {StaticUrlPrefix} = window.config;

export const names = Object.keys(map);
export const giteaImage = `<img alt=":gitea:" title=":gitea:" class="emoji" src="${StaticUrlPrefix}/img/emoji.png" align="absmiddle">`;

const regex = new RegExp(`:(${names.map(escapeStringRegexp).join('|')}):`, 'g');

function stringReplacer(_, name) {
  return map[name];
}

function nodeReplacer(_, match) {
  const name = match[1];
  return map[name];
}

export function replaceEmojiTokens(node) {
  if (!node) return;

  // sometimes this is called on plain strings
  if (typeof node === 'string') {
    return node.replace(regex, stringReplacer);
  }

  // replace :tada: with 🎉
  findAndReplaceDOMText(node, {
    find: regex,
    replace: nodeReplacer,
  });

  // replace :gitea: with image
  findAndReplaceDOMText(node, {
    find: ':gitea:',
    replace: () => {
      const container = document.createElement('div');
      container.innerHTML = giteaImage;
      return container.firstChild;
    }
  });
}
