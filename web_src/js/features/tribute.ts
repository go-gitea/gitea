import {emojiKeys, emojiHTML, emojiString} from './emoji.ts';
import {html, htmlRaw} from '../utils/html.ts';

type TributeItem = Record<string, any>;

export async function attachTribute(element: HTMLElement) {
  const {default: Tribute} = await import(/* webpackChunkName: "tribute" */'tributejs');

  const collections = [
    { // emojis
      trigger: ':',
      requireLeadingSpace: true,
      values: (query: string, cb: (matches: Array<string>) => void) => {
        const matches = [];
        for (const name of emojiKeys) {
          if (name.includes(query)) {
            matches.push(name);
            if (matches.length > 5) break;
          }
        }
        cb(matches);
      },
      lookup: (item: TributeItem) => item,
      selectTemplate: (item: TributeItem) => {
        if (item === undefined) return null;
        return emojiString(item.original);
      },
      menuItemTemplate: (item: TributeItem) => {
        return html`<div class="tribute-item">${htmlRaw(emojiHTML(item.original))}<span>${item.original}</span></div>`;
      },
    }, { // mentions
      values: window.config.mentionValues ?? [],
      requireLeadingSpace: true,
      menuItemTemplate: (item: TributeItem) => {
        const fullNameHtml = item.original.fullname && item.original.fullname !== '' ? html`<span class="fullname">${item.original.fullname}</span>` : '';
        return html`
          <div class="tribute-item">
            <img alt src="${item.original.avatar}" width="21" height="21"/>
            <span class="name">${item.original.name}</span>
            ${htmlRaw(fullNameHtml)}
          </div>
        `;
      },
    },
  ];

  // @ts-expect-error TS2351: This expression is not constructable (strange, why)
  const tribute = new Tribute({collection: collections, noMatchTemplate: ''});
  tribute.attach(element);
  return tribute;
}
