import {sourcesContainElk} from './mermaid.ts';
import {dedent} from '../utils/testhelper.ts';

test('sourcesContainElk', () => {
  expect(sourcesContainElk([dedent(`
    flowchart TB
      elk --> B
  `)])).toEqual(false);

  expect(sourcesContainElk([dedent(`
    ---
    config:
      layout : elk
    ---
    flowchart TB
      A --> B
  `)])).toEqual(true);

  expect(sourcesContainElk([dedent(`
    ---
    config:
      layout: elk.layered
    ---
    flowchart TB
      A --> B
  `)])).toEqual(true);

  expect(sourcesContainElk([`
    %%{ init : { "flowchart": { "defaultRenderer": "elk" } } }%%
    flowchart TB
      A --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    %%{init:{
        "layout" : "elk.layered"
    }}%%
    flowchart TB
      A --> B
  `])).toEqual(true);
});
