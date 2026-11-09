type DiffOpType = 'equal' | 'delete' | 'insert';

const maxEditCost = 1000; // beyond it a range is replaced as a whole, very different content would take too long

// a range of "a" and "b", only one side has items for "delete" and "insert"
export type DiffOp = {type: DiffOpType; aStart: number; aEnd: number; bStart: number; bEnd: number};

function pushOp(ops: DiffOp[], type: DiffOpType, aStart: number, aEnd: number, bStart: number, bEnd: number) {
  if (aStart === aEnd && bStart === bEnd) return;
  const last = ops.at(-1);
  if (last?.type === type && last.aEnd === aStart && last.bEnd === bStart) {
    last.aEnd = aEnd;
    last.bEnd = bEnd;
  } else {
    ops.push({type, aStart, aEnd, bStart, bEnd});
  }
}

// finds the middle snake of Myers' linear space algorithm, and diffs both halves around it
function bisect<T>(ops: DiffOp[], a: T[], b: T[], aLo: number, aHi: number, bLo: number, bHi: number) {
  const len1 = aHi - aLo;
  const len2 = bHi - bLo;
  const maxD = Math.ceil((len1 + len2) / 2);
  const vOffset = maxD;
  const vLength = 2 * maxD;
  const v1 = new Int32Array(vLength).fill(-1);
  const v2 = new Int32Array(vLength).fill(-1);
  v1[vOffset + 1] = 0;
  v2[vOffset + 1] = 0;
  const delta = len1 - len2;
  const front = delta % 2 !== 0;
  let k1start = 0;
  let k1end = 0;
  let k2start = 0;
  let k2end = 0;
  for (let d = 0; d < Math.min(maxD, maxEditCost); d++) {
    for (let k1 = -d + k1start; k1 <= d - k1end; k1 += 2) {
      const k1Offset = vOffset + k1;
      let x1 = k1 === -d || (k1 !== d && v1[k1Offset - 1] < v1[k1Offset + 1]) ? v1[k1Offset + 1] : v1[k1Offset - 1] + 1;
      let y1 = x1 - k1;
      while (x1 < len1 && y1 < len2 && a[aLo + x1] === b[bLo + y1]) {
        x1++;
        y1++;
      }
      v1[k1Offset] = x1;
      if (x1 > len1) {
        k1end += 2;
      } else if (y1 > len2) {
        k1start += 2;
      } else if (front) {
        const k2Offset = vOffset + delta - k1;
        if (k2Offset >= 0 && k2Offset < vLength && v2[k2Offset] !== -1 && x1 >= len1 - v2[k2Offset]) {
          diffRange(ops, a, b, aLo, aLo + x1, bLo, bLo + y1);
          diffRange(ops, a, b, aLo + x1, aHi, bLo + y1, bHi);
          return;
        }
      }
    }
    for (let k2 = -d + k2start; k2 <= d - k2end; k2 += 2) {
      const k2Offset = vOffset + k2;
      let x2 = k2 === -d || (k2 !== d && v2[k2Offset - 1] < v2[k2Offset + 1]) ? v2[k2Offset + 1] : v2[k2Offset - 1] + 1;
      let y2 = x2 - k2;
      while (x2 < len1 && y2 < len2 && a[aHi - x2 - 1] === b[bHi - y2 - 1]) {
        x2++;
        y2++;
      }
      v2[k2Offset] = x2;
      if (x2 > len1) {
        k2end += 2;
      } else if (y2 > len2) {
        k2start += 2;
      } else if (!front) {
        const k1Offset = vOffset + delta - k2;
        if (k1Offset >= 0 && k1Offset < vLength && v1[k1Offset] !== -1) {
          const x1 = v1[k1Offset];
          const y1 = vOffset + x1 - k1Offset;
          if (x1 >= len1 - x2) {
            diffRange(ops, a, b, aLo, aLo + x1, bLo, bLo + y1);
            diffRange(ops, a, b, aLo + x1, aHi, bLo + y1, bHi);
            return;
          }
        }
      }
    }
  }
  pushOp(ops, 'delete', aLo, aHi, bLo, bLo);
  pushOp(ops, 'insert', aHi, aHi, bLo, bHi);
}

function diffRange<T>(ops: DiffOp[], a: T[], b: T[], aLo: number, aHi: number, bLo: number, bHi: number) {
  let prefix = 0;
  while (aLo + prefix < aHi && bLo + prefix < bHi && a[aLo + prefix] === b[bLo + prefix]) prefix++;
  pushOp(ops, 'equal', aLo, aLo + prefix, bLo, bLo + prefix);
  aLo += prefix;
  bLo += prefix;
  let suffix = 0;
  while (aHi - suffix > aLo && bHi - suffix > bLo && a[aHi - suffix - 1] === b[bHi - suffix - 1]) suffix++;
  const aMid = aHi - suffix;
  const bMid = bHi - suffix;
  if (aLo === aMid) {
    pushOp(ops, 'insert', aLo, aLo, bLo, bMid);
  } else if (bLo === bMid) {
    pushOp(ops, 'delete', aLo, aMid, bLo, bLo);
  } else {
    bisect(ops, a, b, aLo, aMid, bLo, bMid);
  }
  pushOp(ops, 'equal', aMid, aHi, bMid, bHi);
}

// Myers' O(ND) difference algorithm in linear space, the result covers both sequences completely
export function diffSequences<T>(a: T[], b: T[]): DiffOp[] {
  const ops: DiffOp[] = [];
  diffRange(ops, a, b, 0, a.length, 0, b.length);
  return ops;
}
