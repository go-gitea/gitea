import {emojiKeys, emojiHTML, emojiString} from './emoji.ts';
import {htmlEscape} from 'escape-goat';

function makeCollections({mentions, emoji}) {
  const collections = [];

  if (emoji) {
    collections.push({
      trigger: ':',
      requireLeadingSpace: true,
      values: (query, cb) => {
        const matches = [];
        for (const name of emojiKeys) {
          if (name.includes(query)) {
            matches.push(name);
            if (matches.length > 5) break;
          }
        }
        cb(matches);
      },
      lookup: (item) => item,
      selectTemplate: (item) => {
        if (item === undefined) return null;
        return emojiString(item.original);
      },
      menuItemTemplate: (item) => {
        return `<div class="tribute-item">${emojiHTML(item.original)}<span>${htmlEscape(item.original)}</span></div>`;
      },
    });
  }

  if (mentions) {
    collections.push({
      values: window.config.mentionValues ?? [],
      requireLeadingSpace: true,
      menuItemTemplate: (item) => {
        return `
          <div class="tribute-item">
            <img src="${htmlEscape(item.original.avatar)}" width="21" height="21"/>
            <span class="name">${htmlEscape(item.original.name)}</span>
            ${item.original.fullname && item.original.fullname !== '' ? `<span class="fullname">${htmlEscape(item.original.fullname)}</span>` : ''}
          </div>
        `;
      },
    });
  }

  return collections;
}

export async function attachTribute(element, {mentions, emoji}) {
  const {default: Tribute} = await import(/* webpackChunkName: "tribute" */'tributejs');
  const collections = makeCollections({mentions, emoji});
  // @ts-expect-error TS2351: This expression is not constructable (strange, why)
  const tribute = new Tribute({collection: collections, noMatchTemplate: ''});
  tribute.attach(element);
  return tribute;
}
