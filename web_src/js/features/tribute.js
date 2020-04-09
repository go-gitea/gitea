import map from 'emojilib/simplemap.json';
import {giteaImage, names} from './emoji.js';

const allNames = [...names, 'gitea'];

export const issuesTribute = new Tribute({
  values: window.config.tributeValues,
  noMatchTemplate() { return null },
  menuItemTemplate(item) {
    const div = $('<div/>');
    div.append($('<img/>', {src: item.original.avatar}));
    div.append($('<span/>', {class: 'name'}).text(item.original.name));
    if (item.original.fullname && item.original.fullname !== '') {
      div.append($('<span/>', {class: 'fullname'}).text(item.original.fullname));
    }
    return div.html();
  }
});

export const emojiTribute = new Tribute({
  collection: [{
    trigger: ':',
    requireLeadingSpace: true,
    values(query, cb) {
      const matches = [];
      for (const name of allNames) {
        if (name.startsWith(query)) {
          matches.push(name);
          if (matches.length > 5) break;
        }
      }
      cb(matches);
    },
    lookup(item) {
      return item;
    },
    selectTemplate(item) {
      if (typeof item === 'undefined') return null;
      return `:${item.original}:`;
    },
    menuItemTemplate(item) {
      if (item.original === 'gitea') {
        return giteaImage;
      }
      return map[item.original] || '';
    }
  }]
});

export function initTribute() {
  if (!window.config.Tribute) return;

  let content = document.getElementById('content');
  if (content !== null) {
    issuesTribute.attach(content);
  }

  const emojiInputs = document.querySelectorAll('.emoji-input');
  if (emojiInputs.length > 0) {
    emojiTribute.attach(emojiInputs);
  }

  content = document.getElementById('content');
  if (content !== null) {
    emojiTribute.attach(document.getElementById('content'));
  }
}
