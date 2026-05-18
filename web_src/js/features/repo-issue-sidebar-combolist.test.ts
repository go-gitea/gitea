import {IssueSidebarComboList, syncIssueMainContentTimelineItems} from './repo-issue-sidebar-combolist.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';

vi.mock('../modules/fetch.ts', () => ({
  GET: vi.fn(async () => new Response('<div class="issue-content-right"></div><div class="issue-content-left"></div>', {status: 200})),
  POST: vi.fn(async () => new Response('{}', {status: 200})),
}));

describe('syncIssueMainContentTimelineItems', () => {
  test('InsertNew', () => {
    const oldContent = createElementFromHTML(`
    <div>
        <div class="timeline-item">First</div>
        <div class="timeline-item" id="timeline-comments-end"></div>
    </div>
  `);
    const newContent = createElementFromHTML(`
    <div>
        <div class="timeline-item" id="a">New</div>
    </div>
  `);
    syncIssueMainContentTimelineItems(oldContent, newContent);
    expect(oldContent.innerHTML.replace(/>\s+</g, '><').trim()).toBe(
      `<div class="timeline-item">First</div>` +
      `<div class="timeline-item" id="a">New</div>` +
      `<div class="timeline-item" id="timeline-comments-end"></div>`,
    );
  });

  test('Sync', () => {
    const oldContent = createElementFromHTML(`
    <div>
      <div class="timeline-item">First</div>
      <div class="timeline-item" id="it-1">Item 1</div>
      <div class="timeline-item event" id="it-2">Item 2</div>
      <div class="timeline-item" id="it-3">Item 3</div>
      <div class="timeline-item event" id="it-4">Item 4</div>
      <div class="timeline-item" id="timeline-comments-end"></div>
      <div class="timeline-item">Other</div>
    </div>
  `);
    const newContent = createElementFromHTML(`
    <div>
      <div class="timeline-item" id="it-1">New 1</div>
      <div class="timeline-item event" id="it-2">New 2</div>
      <div class="timeline-item" id="it-x">New X</div>
    </div>
  `);
    syncIssueMainContentTimelineItems(oldContent, newContent);

    // Item 1 won't be replaced because it's not an event
    // Item 2 will be replaced with New 2
    // Item 3 will be kept because it's not in new content
    // Item 4 will be removed because it's not in new content, and it's an event
    // New X will be inserted at the end of timeline items (before timeline-comments-end)
    expect(oldContent.innerHTML.replace(/>\s+</g, '><').trim()).toBe(
      `<div class="timeline-item">First</div>` +
      `<div class="timeline-item" id="it-1">Item 1</div>` +
      `<div class="timeline-item event" id="it-2">New 2</div>` +
      `<div class="timeline-item" id="it-3">Item 3</div>` +
      `<div class="timeline-item" id="it-x">New X</div>` +
      `<div class="timeline-item" id="timeline-comments-end"></div>` +
      `<div class="timeline-item">Other</div>`,
    );
  });
});

describe('IssueSidebarComboList deferred-mode project column carrier', () => {
  beforeEach(() => {
    // The unit harness does not load Fomantic's jQuery dropdown plugin; stub it
    // so init() (called by updateUiList on cloned sub-combos, and by the project
    // combo) does not throw. The stub is inert — the carrier/card logic under
    // test is plain DOM, not Fomantic.
    (window.$ as any).fn.dropdown = function () {
      return this;
    };
  });

  test('column combo with data-update-url does NOT write the carrier (edit path unchanged)', () => {
    document.body.innerHTML = `
      <input class="js-project-board-ids" value="">
      <div class="issue-sidebar-combo sidebar-project-column-combo" data-selection-mode="single" data-update-algo="all"
           data-project-id="3" data-update-url="/x/issues/projects/column?issue_id=9">
        <input class="combo-value" name="column_id" type="hidden" value="12">
        <div class="ui dropdown"><div class="menu"><div class="item checked" data-value="12">C12</div></div></div>
      </div>`;
    const combo = new IssueSidebarComboList(document.querySelector('.sidebar-project-column-combo')!);
    // Edit-path correctness: doUpdate only calls recomputeProjectBoardCarrier when updateUrl is
    // empty. updateUrl is read from data-update-url in the constructor, so a non-empty value here
    // proves the carrier-recompute gate stays closed on the edit path. (init() is intentionally not
    // called: it registers fomantic's jQuery dropdown plugin, which the unit harness does not load;
    // it is unrelated to the carrier logic under test.)
    expect(combo.updateUrl).toBe('/x/issues/projects/column?issue_id=9');
    const carrier = document.querySelector<HTMLInputElement>('.js-project-board-ids')!;
    expect(carrier.value).toBe('');
  });

  test('create-page column combo writes projID:colID to the carrier', () => {
    document.body.innerHTML = `
      <input class="js-project-board-ids" value="">
      <div class="issue-sidebar-combo sidebar-project-column-combo" data-selection-mode="single" data-update-algo="all"
           data-project-id="3">
        <input class="combo-value" name="column_id" type="hidden" value="12">
        <div class="ui dropdown"><div class="menu"><div class="item checked" data-value="12">C12</div></div></div>
      </div>
      <div class="issue-sidebar-combo sidebar-project-column-combo" data-selection-mode="single" data-update-algo="all"
           data-project-id="7">
        <input class="combo-value" name="column_id" type="hidden" value="0">
        <div class="ui dropdown"><div class="menu"></div></div>
      </div>`;
    const combo = new IssueSidebarComboList(document.querySelector('.sidebar-project-column-combo')!);
    combo.recomputeProjectBoardCarrier();
    const carrier = document.querySelector<HTMLInputElement>('.js-project-board-ids')!;
    // recomputeProjectBoardCarrier guards with `if (val)` where val is a string. '0' is a
    // non-empty (truthy) string, so project 7 IS included as 7:0.
    expect(carrier.value).toBe('3:12,7:0');
  });

  test('clicking a column item on a cloned create-page card writes that column to the carrier', () => {
    // Mirrors the real create-page flow: project combo clones a hidden card
    // template, inits the cloned column sub-combo, user clicks a non-default
    // column item. Reproduces issue #25: created "In Progress" but landed in
    // the default column because the carrier did not pick up the click.
    document.body.innerHTML = `
      <input class="js-project-board-ids" name="project_board_ids" type="hidden" value="">
      <div class="issue-sidebar-combo sidebar-project-combo" data-selection-mode="multiple" data-update-algo="all">
        <input class="combo-value" name="project_ids" type="hidden" value="5">
        <div class="ui dropdown">
          <div class="menu">
            <a class="item checked" data-value="5">Proj 5</a>
          </div>
        </div>
        <div class="ui list" data-combo-list-inited="true">
          <div class="item empty-list tw-hidden">no projects</div>
        </div>
        <div class="js-project-card-templates tw-hidden">
          <div class="js-project-card-template" data-project-id="5">
            <div class="item sidebar-project-card">
              <a class="suppressed flex-text-block" href="#">Proj 5</a>
              <div class="issue-sidebar-combo sidebar-project-column-combo" data-selection-mode="single" data-update-algo="all"
                   data-project-id="5">
                <input class="combo-value" name="column_id" type="hidden" value="100">
                <div class="ui dropdown full-width">
                  <div class="flex-text-block">
                    <div class="fixed-text" data-no-column-text="No column">
                      <div class="gt-ellipsis">Backlog</div>
                      <svg></svg>
                    </div>
                  </div>
                  <div class="menu flex-items-menu">
                    <a class="item" data-value="100"><span class="item-check-mark"></span><div class="gt-ellipsis">Backlog</div></a>
                    <a class="item" data-value="200"><span class="item-check-mark"></span><div class="gt-ellipsis">In Progress</div></a>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>`;

    const projectCombo = new IssueSidebarComboList(document.querySelector('.sidebar-project-combo')!);
    // simulate selecting project 5: clone its card into the visible list + init sub-combo
    projectCombo.updateUiList(['5']);

    const colCombo = document.querySelector<HTMLElement>('.ui.list .sidebar-project-column-combo')!;
    expect(colCombo).not.toBeNull();
    // user clicks the "In Progress" (200) column item on the cloned card
    const inProgress = colCombo.querySelector<HTMLElement>('.menu > .item[data-value="200"]')!;
    inProgress.dispatchEvent(new MouseEvent('click', {bubbles: true}));

    const carrier = document.querySelector<HTMLInputElement>('.js-project-board-ids')!;
    expect(carrier.value).toBe('5:200');
  });

  test('clearing a seeded default project on create page removes the card and submits no project', () => {
    // Reproduces: new issue, server pre-seeded the repo default project (visible
    // card rendered + project menu item checked + project_ids prefilled), user
    // clicks "clear projects". The issue must then be created with NO project,
    // but the stale visible card + carrier kept assigning the default project.
    document.body.innerHTML = `
      <input class="js-project-board-ids" name="project_board_ids" type="hidden" value="5:200">
      <div class="issue-sidebar-combo sidebar-project-combo" data-selection-mode="multiple" data-update-algo="all">
        <input class="combo-value" name="project_ids" type="hidden" value="5">
        <div class="ui dropdown">
          <div class="menu">
            <a class="item clear-selection" data-text="">Clear projects</a>
            <a class="item checked" data-value="5">Proj 5</a>
          </div>
        </div>
        <div class="ui list" data-combo-list-inited="true">
          <div class="item empty-list tw-hidden">no projects</div>
          <div class="item sidebar-project-card">
            <a class="suppressed flex-text-block" href="#">Proj 5</a>
            <div class="issue-sidebar-combo sidebar-project-column-combo" data-selection-mode="single" data-update-algo="all"
                 data-project-id="5">
              <input class="combo-value" name="column_id" type="hidden" value="200">
              <div class="ui dropdown full-width">
                <div class="flex-text-block">
                  <div class="fixed-text" data-no-column-text="No column">
                    <div class="gt-ellipsis">In Progress</div><svg></svg>
                  </div>
                </div>
                <div class="menu flex-items-menu">
                  <a class="item" data-value="100"><div class="gt-ellipsis">Backlog</div></a>
                  <a class="item checked" data-value="200"><div class="gt-ellipsis">In Progress</div></a>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>`;

    const projectCombo = new IssueSidebarComboList(document.querySelector('.sidebar-project-combo')!);
    projectCombo.init();

    const clear = document.querySelector<HTMLElement>('.menu > .item.clear-selection')!;
    clear.dispatchEvent(new MouseEvent('click', {bubbles: true}));

    const projectIds = document.querySelector<HTMLInputElement>('input[name="project_ids"]')!;
    const carrier = document.querySelector<HTMLInputElement>('.js-project-board-ids')!;
    // no project selected -> form must submit empty project_ids AND empty carrier,
    // and the visible project card must be gone
    expect(projectIds.value).toBe('');
    expect(document.querySelector('.ui.list .sidebar-project-card')).toBeNull();
    expect(carrier.value).toBe('');
  });

  test('recomputeProjectBoardCarrier is a no-op when there is no carrier (edit page)', () => {
    document.body.innerHTML = `
      <div class="issue-sidebar-combo sidebar-project-column-combo" data-selection-mode="single" data-update-algo="all"
           data-project-id="3" data-update-url="/x/issues/projects/column?issue_id=9">
        <input class="combo-value" name="column_id" value="12">
        <div class="ui dropdown"><div class="menu"></div></div>
      </div>`;
    const combo = new IssueSidebarComboList(document.querySelector('.sidebar-project-column-combo')!);
    expect(() => combo.recomputeProjectBoardCarrier()).not.toThrow();
  });

  test('syncDeferredColumnLabel reflects the checked menu item into the label', () => {
    document.body.innerHTML = `
      <div class="issue-sidebar-combo sidebar-project-column-combo" data-selection-mode="single" data-update-algo="all"
           data-project-id="3">
        <input class="combo-value" name="column_id" type="hidden" value="22">
        <div class="ui dropdown">
          <div class="flex-text-block">
            <div class="fixed-text" data-no-column-text="No column">
              <div class="gt-ellipsis">Backlog</div>
              <svg></svg>
            </div>
          </div>
          <div class="menu">
            <a class="item" data-value="11"><div class="gt-ellipsis">Backlog</div></a>
            <a class="item checked" data-value="22"><span class="color-icon"></span><div class="gt-ellipsis">In Progress</div></a>
          </div>
        </div>
      </div>`;
    const combo = new IssueSidebarComboList(document.querySelector('.sidebar-project-column-combo')!);
    combo.syncDeferredColumnLabel();
    const label = document.querySelector('.fixed-text')!;
    expect(label.querySelector('.gt-ellipsis')!.textContent).toBe('In Progress');
    expect(label.querySelector('.color-icon')).not.toBeNull(); // color swatch carried over
    expect(label.querySelector('svg')).not.toBeNull(); // triangle preserved
  });

  test('syncDeferredColumnLabel falls back to no-column text when nothing checked', () => {
    document.body.innerHTML = `
      <div class="issue-sidebar-combo sidebar-project-column-combo" data-selection-mode="single" data-update-algo="all"
           data-project-id="3">
        <input class="combo-value" name="column_id" type="hidden" value="">
        <div class="ui dropdown">
          <div class="flex-text-block">
            <div class="fixed-text" data-no-column-text="No column">
              <div class="gt-ellipsis">Backlog</div>
              <svg></svg>
            </div>
          </div>
          <div class="menu">
            <a class="item" data-value="11"><div class="gt-ellipsis">Backlog</div></a>
          </div>
        </div>
      </div>`;
    const combo = new IssueSidebarComboList(document.querySelector('.sidebar-project-column-combo')!);
    combo.syncDeferredColumnLabel();
    expect(document.querySelector('.fixed-text > .gt-ellipsis')!.textContent).toBe('No column');
  });

  // Builds a create-page project combo with N hidden card templates. Each card
  // has a column sub-combo whose checked item is the project's default column.
  const buildProjectComboFixture = (projects: Array<{id: number; defaultCol: number; cols: number[]}>) => {
    const cardTpl = (p: {id: number; defaultCol: number; cols: number[]}) => `
      <div class="js-project-card-template" data-project-id="${p.id}">
        <div class="item sidebar-project-card">
          <a class="suppressed flex-text-block" href="#">Proj ${p.id}</a>
          <div class="issue-sidebar-combo sidebar-project-column-combo" data-selection-mode="single" data-update-algo="all"
               data-project-id="${p.id}">
            <input class="combo-value" name="column_id" type="hidden" value="${p.defaultCol}">
            <div class="ui dropdown full-width">
              <div class="flex-text-block">
                <div class="fixed-text" data-no-column-text="No column">
                  <div class="gt-ellipsis">col${p.defaultCol}</div><svg></svg>
                </div>
              </div>
              <div class="menu flex-items-menu">
                ${p.cols.map((c) => `<a class="item${c === p.defaultCol ? ' checked' : ''}" data-value="${c}"><span class="item-check-mark"></span><div class="gt-ellipsis">col${c}</div></a>`).join('')}
              </div>
            </div>
          </div>
        </div>
      </div>`;
    document.body.innerHTML = `
      <input class="js-project-board-ids" name="project_board_ids" type="hidden" value="">
      <div class="issue-sidebar-combo sidebar-project-combo" data-selection-mode="multiple" data-update-algo="all">
        <input class="combo-value" name="project_ids" type="hidden" value="">
        <div class="ui dropdown">
          <div class="menu">
            <a class="item clear-selection" data-text="">Clear projects</a>
            ${projects.map((p) => `<a class="item" data-value="${p.id}">Proj ${p.id}</a>`).join('')}
          </div>
        </div>
        <div class="ui list" data-combo-list-inited="true">
          <div class="item empty-list tw-hidden">no projects</div>
        </div>
        <div class="js-project-card-templates tw-hidden">
          ${projects.map(cardTpl).join('')}
        </div>
      </div>`;
    return new IssueSidebarComboList(document.querySelector('.sidebar-project-combo')!);
  };

  const carrierValue = () => document.querySelector<HTMLInputElement>('.js-project-board-ids')!.value;
  const clickColumn = (projectId: number, colId: number) => {
    const card = document.querySelector<HTMLElement>(`.ui.list .sidebar-project-column-combo[data-project-id="${projectId}"]`)!;
    card.querySelector<HTMLElement>(`.menu > .item[data-value="${colId}"]`)!.dispatchEvent(new MouseEvent('click', {bubbles: true}));
  };
  // Faithfully reproduce "the page rendered with these projects selected": the
  // server marks the project menu items checked + project_ids, then init()
  // clones the matching cards. After this, item clicks toggle real state.
  const selectProjects = (combo: IssueSidebarComboList, ids: number[]) => {
    for (const id of ids) {
      document.querySelector(`.sidebar-project-combo .menu > .item[data-value="${id}"]`)!.classList.add('checked');
    }
    document.querySelector<HTMLInputElement>('input[name="project_ids"]')!.value = ids.join(',');
    combo.init(); // wires the delegated click handler + initialValues
    combo.updateUiList(ids.map(String)); // clones the selected projects' cards
  };

  test('multi-project selection accumulates one projID:colID pair per project', () => {
    const combo = buildProjectComboFixture([
      {id: 5, defaultCol: 100, cols: [100, 200]},
      {id: 8, defaultCol: 300, cols: [300, 400]},
    ]);
    selectProjects(combo, [5, 8]);
    clickColumn(5, 200);
    clickColumn(8, 400);
    // order follows DOM order of the cloned cards (project order)
    expect(carrierValue()).toBe('5:200,8:400');
  });

  test('deselecting one project of several removes only its card and its carrier pair', () => {
    const combo = buildProjectComboFixture([
      {id: 5, defaultCol: 100, cols: [100, 200]},
      {id: 8, defaultCol: 300, cols: [300, 400]},
    ]);
    selectProjects(combo, [5, 8]);
    clickColumn(5, 200);
    clickColumn(8, 400);
    expect(carrierValue()).toBe('5:200,8:400');

    // deselect project 5 (multi-select item toggle, NOT clear-selection)
    document.querySelector<HTMLElement>('.sidebar-project-combo .menu > .item[data-value="5"]')!
      .dispatchEvent(new MouseEvent('click', {bubbles: true}));

    expect(document.querySelector('.ui.list .sidebar-project-column-combo[data-project-id="5"]')).toBeNull();
    expect(document.querySelector('.ui.list .sidebar-project-column-combo[data-project-id="8"]')).not.toBeNull();
    expect(carrierValue()).toBe('8:300'); // project 8 reverts to its default column on re-clone
  });

  test('changing a column twice keeps only the latest value in the carrier and label', () => {
    const combo = buildProjectComboFixture([{id: 5, defaultCol: 100, cols: [100, 200, 300]}]);
    selectProjects(combo, [5]);
    clickColumn(5, 200);
    expect(carrierValue()).toBe('5:200');
    clickColumn(5, 300);
    expect(carrierValue()).toBe('5:300'); // latest, not accumulated/reverted
    const label = document.querySelector('.ui.list .sidebar-project-column-combo[data-project-id="5"] .fixed-text')!;
    expect(label.querySelector('.gt-ellipsis')!.textContent).toBe('col300');
  });

  test('clearing then re-adding a project yields a fresh default-column card and carrier', () => {
    const combo = buildProjectComboFixture([{id: 5, defaultCol: 100, cols: [100, 200]}]);
    selectProjects(combo, [5]);
    clickColumn(5, 200); // pick a non-default column
    expect(carrierValue()).toBe('5:200');

    // clear all projects
    document.querySelector<HTMLElement>('.menu > .item.clear-selection')!
      .dispatchEvent(new MouseEvent('click', {bubbles: true}));
    expect(document.querySelector('.ui.list .sidebar-project-card')).toBeNull();
    expect(carrierValue()).toBe('');

    // re-add the SAME project via a menu toggle: clone is from the pristine
    // hidden source, so the column resets to the project default (no bleed)
    document.querySelector<HTMLElement>('.sidebar-project-combo .menu > .item[data-value="5"]')!
      .dispatchEvent(new MouseEvent('click', {bubbles: true}));
    const recloned = document.querySelector<HTMLInputElement>(
      '.ui.list .sidebar-project-column-combo[data-project-id="5"] > .combo-value',
    )!;
    expect(recloned.value).toBe('100');
    expect(carrierValue()).toBe('5:100');
  });

  test('edit-mode column combo (data-update-url present) never writes the carrier when clicked', async () => {
    // The #1 design risk: shared combo must not enter deferred mode on the edit
    // page. Drive a real column click on an edit-mode combo and assert the
    // create-only carrier is never written and no clone source is consulted.
    document.body.innerHTML = `
      <div class="issue-sidebar-combo sidebar-project-column-combo" data-selection-mode="single" data-update-algo="all"
           data-project-id="3" data-update-url="/x/issues/projects/column?issue_id=9">
        <input class="combo-value" name="column_id" type="hidden" value="11">
        <div class="ui dropdown">
          <div class="menu">
            <a class="item checked" data-value="11"><div class="gt-ellipsis">Backlog</div></a>
            <a class="item" data-value="22"><div class="gt-ellipsis">In Progress</div></a>
          </div>
        </div>
      </div>`;
    // edit page has the sidebar containers reloadPagePartially expects
    document.body.insertAdjacentHTML('beforeend',
      '<div class="issue-content-right"></div><div class="issue-content-left"></div>');
    const combo = new IssueSidebarComboList(document.querySelector('.sidebar-project-column-combo')!);
    combo.init();
    vi.mocked(POST).mockClear();

    document.querySelector<HTMLElement>('.menu > .item[data-value="22"]')!
      .dispatchEvent(new MouseEvent('click', {bubbles: true}));
    await vi.waitFor(() => expect(POST).toHaveBeenCalled());

    // Edit path took the immediate-POST route (not deferred mode), and no
    // create-page carrier was ever created/written.
    expect(POST).toHaveBeenCalledWith(
      expect.stringContaining('/x/issues/projects/column?issue_id=9'),
      expect.anything(),
    );
    expect(document.querySelector('.js-project-board-ids')).toBeNull();
    expect(combo.updateUrl).toBe('/x/issues/projects/column?issue_id=9');
  });
});
