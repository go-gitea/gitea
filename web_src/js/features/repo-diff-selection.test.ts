import {applyDiffLineSelection} from './repo-diff-selection.ts';

function createDiffRow(tbody: HTMLTableSectionElement, options: {id?: string, lineType?: string} = {}) {
  const tr = document.createElement('tr');
  if (options.lineType) tr.setAttribute('data-line-type', options.lineType);

  const numberCell = document.createElement('td');
  numberCell.classList.add('lines-num');
  const span = document.createElement('span');
  if (options.id) span.id = options.id;
  numberCell.append(span);
  tr.append(numberCell);

  tr.append(document.createElement('td'));
  tbody.append(tr);
  return tr;
}

describe('applyDiffLineSelection', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  test('selects contiguous diff rows, skips expansion rows, and clears previous selection', () => {
    const fragment = 'diff-selection';

    const otherBox = document.createElement('div');
    const otherTable = document.createElement('table');
    otherTable.classList.add('code-diff');
    const otherTbody = document.createElement('tbody');
    const staleActiveRow = document.createElement('tr');
    staleActiveRow.classList.add('active');
    otherTbody.append(staleActiveRow);
    otherTable.append(otherTbody);
    otherBox.append(otherTable);

    const container = document.createElement('div');
    container.classList.add('diff-file-box');
    const table = document.createElement('table');
    table.classList.add('code-diff');
    const tbody = document.createElement('tbody');
    table.append(tbody);
    container.append(table);

    const rows = [
      createDiffRow(tbody, {id: `${fragment}L1`}),
      createDiffRow(tbody),
      createDiffRow(tbody, {lineType: '4'}),
      createDiffRow(tbody),
      createDiffRow(tbody, {id: `${fragment}R5`}),
      createDiffRow(tbody),
    ];

    document.body.append(otherBox, container);

    const range = {fragment, startSide: 'L' as const, startLine: 1, endSide: 'R' as const, endLine: 5};
    const applied = applyDiffLineSelection(container, range);

    expect(applied).toBe(true);
    expect(rows[0].classList.contains('active')).toBe(true);
    expect(rows[1].classList.contains('active')).toBe(true);
    expect(rows[2].classList.contains('active')).toBe(false);
    expect(rows[3].classList.contains('active')).toBe(true);
    expect(rows[4].classList.contains('active')).toBe(true);
    expect(rows[5].classList.contains('active')).toBe(false);
    expect(staleActiveRow.classList.contains('active')).toBe(false);
  });

  test('returns false when either anchor is missing', () => {
    const fragment = 'diff-missing';
    const container = document.createElement('div');
    container.classList.add('diff-file-box');
    const table = document.createElement('table');
    table.classList.add('code-diff');
    const tbody = document.createElement('tbody');
    table.append(tbody);
    container.append(table);
    document.body.append(container);

    createDiffRow(tbody, {id: `${fragment}L1`});

    const applied = applyDiffLineSelection(container, {fragment, startSide: 'L' as const, startLine: 1, endSide: 'R' as const, endLine: 2});
    expect(applied).toBe(false);
    expect(container.querySelectorAll('tr.active').length).toBe(0);
  });
});
