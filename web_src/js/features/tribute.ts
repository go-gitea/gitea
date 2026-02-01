import {emojiKeys, emojiHTML, emojiString} from './emoji.ts';
import {html, htmlRaw} from '../utils/html.ts';
import type {TributeCollection} from 'tributejs';

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
      lookup: String,
      selectTemplate: (item: TributeItem) => {
        if (item === undefined) return null;
        return emojiString(item.original);
      },
      menuItemTemplate: (item: TributeItem) => {
        return html`<div class="tribute-item">${htmlRaw(emojiHTML(item.original))}<span>${item.original}</span></div>`;
      },
    }, { // mentions
      values: window.config.mentionValues,
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

  const tribute = new Tribute({collection: collections as unknown as TributeCollection<TributeItem>[], noMatchTemplate: ''});
  tribute.attach(element);
  return tribute;
}
