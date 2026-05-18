import {initDefaultProjectScopeFilter} from './repo-settings.ts';

// The pre-selected-project dropdowns are scope-filtered client-side by the
// projects mode (UX only; the server re-validates). Allowed scopes:
//   repo -> {repo}, owner -> {owner}, all -> {repo,owner}, none -> {}
// The value-0 ("None") option has no scope and must always remain visible.
describe('initDefaultProjectScopeFilter', () => {
  beforeEach(() => {
    // The unit harness does not load Fomantic; stub the jQuery dropdown plugin
    // used only on the "selected option became hidden -> reset to None" branch.
    (window.$ as any).fn.dropdown = function () {
      return this;
    };
  });

  const fixture = (mode: string, selectedIssues = '0') => {
    document.body.innerHTML = `
      <div class="page-content repository settings">
        <select name="projects_mode">
          <option value="repo"${mode === 'repo' ? ' selected' : ''}>repo</option>
          <option value="owner"${mode === 'owner' ? ' selected' : ''}>owner</option>
          <option value="all"${mode === 'all' ? ' selected' : ''}>all</option>
          <option value="none"${mode === 'none' ? ' selected' : ''}>none</option>
        </select>
        <div class="field">
          <select name="default_project_id_for_issues">
            <option value="0"${selectedIssues === '0' ? ' selected' : ''}>None</option>
            <option value="11" data-project-scope="repo"${selectedIssues === '11' ? ' selected' : ''}>Repo Board</option>
            <option value="22" data-project-scope="owner"${selectedIssues === '22' ? ' selected' : ''}>Owner Board</option>
          </select>
          <div class="ui dropdown">
            <div class="menu">
              <div class="item" data-value="0">None</div>
              <div class="item" data-value="11">Repo Board</div>
              <div class="item" data-value="22">Owner Board</div>
            </div>
          </div>
        </div>
        <div class="field">
          <select name="default_project_id_for_pull_requests">
            <option value="0" selected>None</option>
            <option value="33" data-project-scope="repo">PR Repo Board</option>
            <option value="44" data-project-scope="owner">PR Owner Board</option>
          </select>
          <div class="ui dropdown">
            <div class="menu">
              <div class="item" data-value="0">None</div>
              <div class="item" data-value="33">PR Repo Board</div>
              <div class="item" data-value="44">PR Owner Board</div>
            </div>
          </div>
        </div>
      </div>`;
    // Set selection via the .value property so option.selected reflects it
    // (happy-dom does not always sync the `selected` attribute to the property).
    document.querySelector<HTMLSelectElement>('select[name="projects_mode"]')!.value = mode;
    document.querySelector<HTMLSelectElement>('select[name="default_project_id_for_issues"]')!.value = selectedIssues;
    return document.querySelector<HTMLElement>('.page-content')!;
  };

  const opt = (name: string, value: string) =>
    document.querySelector<HTMLOptionElement>(`select[name="${name}"] option[value="${value}"]`)!;
  const item = (fieldIndex: number, value: string) =>
    document.querySelectorAll('.field')[fieldIndex].querySelector<HTMLElement>(`.menu .item[data-value="${value}"]`)!;
  const setMode = (value: string) => {
    const sel = document.querySelector<HTMLSelectElement>('select[name="projects_mode"]')!;
    sel.value = value;
    sel.dispatchEvent(new Event('change'));
  };

  test('repo mode hides owner-scope options and their dropdown items', () => {
    initDefaultProjectScopeFilter(fixture('repo'));
    setMode('repo');
    expect(opt('default_project_id_for_issues', '11').hidden).toBe(false); // repo scope kept
    expect(opt('default_project_id_for_issues', '22').hidden).toBe(true); // owner scope hidden
    expect(item(0, '22').classList.contains('tw-hidden')).toBe(true);
    expect(item(0, '11').classList.contains('tw-hidden')).toBe(false);
    // PR dropdown filtered by the same rule
    expect(opt('default_project_id_for_pull_requests', '44').hidden).toBe(true);
    expect(opt('default_project_id_for_pull_requests', '33').hidden).toBe(false);
  });

  test('all mode keeps both repo- and owner-scope options', () => {
    initDefaultProjectScopeFilter(fixture('all'));
    setMode('all');
    expect(opt('default_project_id_for_issues', '11').hidden).toBe(false);
    expect(opt('default_project_id_for_issues', '22').hidden).toBe(false);
  });

  test('the "None" (value 0) option is always visible regardless of mode', () => {
    initDefaultProjectScopeFilter(fixture('repo'));
    for (const mode of ['repo', 'owner', 'all', 'none']) {
      setMode(mode);
      expect(opt('default_project_id_for_issues', '0').hidden).toBe(false);
      expect(item(0, '0').classList.contains('tw-hidden')).toBe(false);
    }
  });

  test('switching mode so the selected option becomes hidden resets the select to None', () => {
    // start with an owner-scope project selected, then switch to repo mode:
    // the owner option is no longer allowed -> select must fall back to "0".
    initDefaultProjectScopeFilter(fixture('owner', '22'));
    setMode('repo');
    const sel = document.querySelector<HTMLSelectElement>('select[name="default_project_id_for_issues"]')!;
    expect(sel.value).toBe('0');
    expect(opt('default_project_id_for_issues', '22').hidden).toBe(true);
  });
});
