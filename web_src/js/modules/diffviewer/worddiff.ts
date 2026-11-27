import {diffArrays} from 'diff';

export type CharRange = [start: number, end: number];

const tokenRe = /[\p{L}\p{N}_]+|\s+|./gsu;
const maxTokens = 2000; // the diff gets slow and its highlights useless for very long lines like minified code

// the changed words of a deleted line and its added counterpart, null if the lines are too long to compare
export function wordDiffRanges(oldText: string, newText: string): {old: CharRange[]; new: CharRange[]} | null {
  const oldTokens = oldText.match(tokenRe) ?? [];
  const newTokens = newText.match(tokenRe) ?? [];
  if (oldTokens.length > maxTokens || newTokens.length > maxTokens) return null;
  const result: {old: CharRange[]; new: CharRange[]} = {old: [], new: []};
  let oldPos = 0;
  let newPos = 0;
  for (const change of diffArrays(oldTokens, newTokens)) {
    const length = change.value.reduce((sum, token) => sum + token.length, 0);
    if (change.removed) {
      result.old.push([oldPos, oldPos + length]);
    } else if (change.added) {
      result.new.push([newPos, newPos + length]);
    }
    if (!change.added) oldPos += length;
    if (!change.removed) newPos += length;
  }
  return result;
}
