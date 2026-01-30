import {sourcesContainElk} from './mermaid.ts';

test('sourcesContainElk', () => {
  expect(sourcesContainElk([`
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(false);

  expect(sourcesContainElk([`
    ---
    config:
      layout: elk
      flowchart:
        defaultRenderer: elk
    ---
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
    %%{ init: { "flowchart": { "defaultRenderer": "elk" } } }%%
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);

  expect(sourcesContainElk([`
    %%{
      init: {
        "layout": "elk",
      }
    }%%
    flowchart TB
      A --> B
      A --> C --> B
  `])).toEqual(true);
});
