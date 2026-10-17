import {diffSequences} from './myers.ts';

export type CharRange = [start: number, end: number];

const tokenRe = /[\p{L}\p{N}_]+|\s+|./gsu;
const maxTokens = 2000; // the diff gets slow and its highlights useless for very long lines like minified code

function tokenOffsets(tokens: string[]): number[] {
  const offsets = [0];
  for (const token of tokens) offsets.push(offsets.at(-1)! + token.length);
  return offsets;
}

// the changed words of a deleted line and its added counterpart, null if the lines are too long to compare
export function wordDiffRanges(oldText: string, newText: string): {old: CharRange[]; new: CharRange[]} | null {
  const oldTokens = oldText.match(tokenRe) ?? [];
  const newTokens = newText.match(tokenRe) ?? [];
  if (oldTokens.length > maxTokens || newTokens.length > maxTokens) return null;
  const oldOffsets = tokenOffsets(oldTokens);
  const newOffsets = tokenOffsets(newTokens);
  const result: {old: CharRange[]; new: CharRange[]} = {old: [], new: []};
  for (const op of diffSequences(oldTokens, newTokens)) {
    if (op.type === 'delete') result.old.push([oldOffsets[op.aStart], oldOffsets[op.aEnd]]);
    else if (op.type === 'insert') result.new.push([newOffsets[op.bStart], newOffsets[op.bEnd]]);
  }
  return result;
}
