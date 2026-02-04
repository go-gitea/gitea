import {sourcesContainElk} from './mermaid.ts';

test('sourcesContainElk', () => {
  expect(sourcesContainElk([`
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(false);

  expect(sourcesContainElk([`
    flowchart TB
      elk --> B
      elk --> C --> B
  `])).toEqual(false);

  expect(sourcesContainElk([`
    ---
    config:
      layout : elk
    ---
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    ---
    config:
      layout: elk.layered
    ---
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    ---
    config:
      "layout": "elk.layered"
    ---
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    ---
    config:
      'layout': 'elk.layered'
    ---
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    ---
    config:
      flowchart:
        defaultRenderer: elk
    ---
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    ---
    config:
      layout: noelk
    ---
    %%{ init: { "layout": "elk" } }%%
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    %%{ init: { "layout": "elk" } }%%
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    %%{ init: { "layout": "elkx" } }%%
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(false);

  expect(sourcesContainElk([`
    %%{ init: { "flowchart": { "defaultRenderer": "elk" } } }%%
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    %%{
      init: {
        "layout": "elk"
      }
    }%%
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    %%{
      init: {
        "layout" : "elk.layered"
      }
    }%%
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    %%{
      init: {
        "flowchart": {
          "defaultRenderer": "elk"
        }
      }
    }%%
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);
});
