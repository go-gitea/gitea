import {emojiKeys, emojiHTML, emojiString} from './emoji.js';

function createMentionsTribute(Tribute) {
  return new Tribute({
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
}

function createEmojiTribute(Tribute) {
  return new Tribute({
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
  });
}

export async function attachTribute(elementOrNodeList, {mentions, emoji} = {}) {
  if (!window.config.Tribute || !elementOrNodeList) return;
  const nodes = Array.from('length' in elementOrNodeList ? elementOrNodeList : [elementOrNodeList]);
  if (!nodes.length) return;

  const {default: Tribute} = await import(/* webpackChunkName: "tribute" */'tributejs');

  const mentionNodes = nodes.filter((node) => mentions || node.id === 'content');
  const emojiNodes = nodes.filter((node) => emoji || node.classList.contains('emoji-input'));
  const mentionTribute = mentionNodes.length && createMentionsTribute(Tribute);
  const emojiTribute = emojiNodes.length && createEmojiTribute(Tribute);

  for (const node of mentionNodes || []) {
    mentionTribute.attach(node);
  }

  for (const node of emojiNodes || []) {
    emojiTribute.attach(node);
  }
}
