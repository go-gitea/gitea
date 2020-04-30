import {emojiKeys, emojiHTML, emojiString} from './emoji.js';

export const issuesTribute = window.config.Tribute ? new Tribute({
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
}) : null;

export const emojiTribute = window.config.Tribute ? new Tribute({
  collection: [{
    trigger: ':',
    requireLeadingSpace: true,
    values(query, cb) {
      const matches = [];
      for (const name of emojiKeys) {
        if (name.includes(query)) {
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
      return emojiString(item.original);
    },
    menuItemTemplate(item) {
      return `<div class="tribute-item">${emojiHTML(item.original)}<span>${item.original}</span></div>`;
    }
  }]
}) : null;

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
