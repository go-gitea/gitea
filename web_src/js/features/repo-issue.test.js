import {describe, expect, test} from 'vitest';
import {excludeLabel} from './repo-issue.js';

describe('Repo Issue', () => {
  test('excludeLabel', () => {
    expect(excludeLabel('?&labels=1&assignee=0', 1)).toEqual('?&labels=-1&assignee=0');
    expect(excludeLabel('?q=&labels=7%2c1&assignee=0', 1)).toEqual('?q=&labels=7%2c-1&assignee=0');
    expect(excludeLabel('?q=&labels=7%2c-3%2c4&poster=0', 4)).toEqual('?q=&labels=7%2c-3%2c-4&poster=0');
    expect(excludeLabel('?q=&labels=15%2c4&poster=0', 5)).toEqual('?q=&labels=15%2c4&poster=0'); // labelId does not exist in href
  });
});

