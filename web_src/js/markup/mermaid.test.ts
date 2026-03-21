import {sourceNeedsElk} from './mermaid.ts';
import {dedent} from '../utils/testhelper.ts';

test('MermaidConfigLayoutCheck', () => {
  expect(sourceNeedsElk(dedent(`
    flowchart TB
      elk --> B
  `))).toEqual(false);

  expect(sourceNeedsElk(dedent(`
    ---
    config:
      layout : elk
    ---
    flowchart TB
      A --> B
  `))).toEqual(true);

  expect(sourceNeedsElk(dedent(`
    ---
    config:
      layout: elk.layered
    ---
    flowchart TB
      A --> B
  `))).toEqual(true);

  expect(sourceNeedsElk(`
    %%{ init : { "flowchart": { "defaultRenderer": "elk" } } }%%
    flowchart TB
      A --> B
  `)).toEqual(true);

  expect(sourceNeedsElk(dedent(`
    ---
    config:
      layout: 123
    ---
    %%{ init : { "class": { "defaultRenderer": "elk.any" } } }%%
    flowchart TB
      A --> B
  `))).toEqual(true);

  expect(sourceNeedsElk(`
    %%{init:{
        "layout" : "elk.layered"
    }}%%
    flowchart TB
      A --> B
  `)).toEqual(true);

  expect(sourceNeedsElk(`
    %%{ initialize: {
        'layout' : 'elk.layered'
    }}%%
    flowchart TB
      A --> B
  `)).toEqual(true);
});
