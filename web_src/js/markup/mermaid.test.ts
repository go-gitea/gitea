import {sourcesContainElk} from './mermaid.ts';

test('sourcesContainElk', () => {
  expect(sourcesContainElk([`
flowchart TB
  elk --> B
`])).toEqual(false);

  expect(sourcesContainElk([`
---
config:
  layout : elk
---
flowchart TB
  A --> B
`.trim()])).toEqual(true);

  expect(sourcesContainElk([`
---
config:
  layout: elk.layered
---
flowchart TB
  A --> B
`.trim()])).toEqual(true);

  expect(sourcesContainElk([`
    %%{ init: { "flowchart": { "defaultRenderer": "elk" } } }%%
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
