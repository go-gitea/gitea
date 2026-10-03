export function moveWithinColumn(column: number[], lane: number[], issueID: number): number[] {
  const result = column.filter((id) => id !== issueID);
  const position = lane.indexOf(issueID);
  const next = lane[position + 1];
  const previous = lane[position - 1];
  let target = result.length;
  if (next !== undefined) {
    target = result.indexOf(next);
  } else if (previous !== undefined) {
    target = result.indexOf(previous) + 1;
  }
  result.splice(target, 0, issueID);
  return result;
}
