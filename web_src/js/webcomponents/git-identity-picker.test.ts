import {GET} from '../modules/fetch.ts';
import './git-identity-picker.ts';

const settingsStore = vi.hoisted(() => new Map<string, unknown>());

vi.mock('../modules/fetch.ts', () => ({GET: vi.fn()}));
vi.mock('../modules/user-settings.ts', () => ({
  localUserSettings: {
    getJsonObject: (key: string, def: unknown) => settingsStore.get(key) ?? def,
    setJsonObject: (key: string, value: unknown) => settingsStore.set(key, value),
  },
}));

const mockedGet = vi.mocked(GET);

type Picker = HTMLElement;

function createPicker(attrs: Record<string, string> = {}): Picker {
  const picker = document.createElement('gitea-git-identity-picker');
  const defaults: Record<string, string> = {
    role: 'author',
    fieldPrefix: 'commit',
    defaultName: 'Default Name',
    defaultEmail: 'primary@example.com',
    customLabel: 'Custom',
    placeholder: 'Name <email>',
    errorFormat: 'Invalid format',
    errorUnverified: 'Unverified email',
    errorLoad: 'Could not load emails',
  };
  for (const [key, value] of Object.entries({...defaults, ...attrs})) {
    const attribute = `data-${key.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`)}`;
    picker.setAttribute(attribute, value);
  }
  document.body.append(picker);
  return picker;
}

function response(records: unknown[], ok = true) {
  return {ok, status: ok ? 200 : 500, json: async () => records} as Response;
}

describe('git identity picker', {concurrent: false}, () => {
  beforeEach(() => {
    document.body.replaceChildren();
    settingsStore.clear();
    mockedGet.mockReset();
  });

  test('uses the primary email by default and renders a custom dropdown', async () => {
    mockedGet.mockResolvedValue(response([
      {email: 'secondary@example.com', verified: true, primary: false},
      {email: 'primary@example.com', verified: true, primary: true},
      {email: 'unverified@example.com', verified: false, primary: false},
    ]));
    const picker = createPicker();

    await vi.waitFor(() => expect(picker.textContent).toContain('secondary@example.com'));
    expect(picker.querySelector('select')).toBeNull();
    expect(picker.querySelector('input[name="commit_name"]')!.getAttribute('type')).toBe('hidden');
    expect(picker.querySelector('button[aria-haspopup="listbox"]')!.textContent).toContain('Default Name <primary@example.com>');
    expect((picker.querySelector('input[name="commit_email"]') as HTMLInputElement).value).toBe('primary@example.com');
    expect(picker.textContent).not.toContain('unverified@example.com');
    expect(mockedGet).toHaveBeenCalledWith('/user/emails');
  });

  test('uses the most recently used valid identity before the primary email', async () => {
    settingsStore.set('git-identity:author:v3', {
      identities: [{name: 'Recent Name', email: 'secondary@example.com'}],
    });
    mockedGet.mockResolvedValue(response([
      {email: 'primary@example.com', verified: true, primary: true},
      {email: 'secondary@example.com', verified: true, primary: false},
    ]));
    const picker = createPicker();

    await vi.waitFor(() => expect((picker.querySelector('input[name="commit_name"]') as HTMLInputElement).value).toBe('Recent Name'));
    expect((picker.querySelector('input[name="commit_email"]') as HTMLInputElement).value).toBe('secondary@example.com');
  });

  test('replaces the dropdown with a custom identity input and validates it', async () => {
    mockedGet.mockResolvedValue(response([{email: 'primary@example.com', verified: true, primary: true}]));
    const picker = createPicker();
    await vi.waitFor(() => expect(picker.querySelectorAll('[role="option"]')).toHaveLength(2));

    (picker.querySelector('button[aria-haspopup="listbox"]') as HTMLButtonElement).click();
    (picker.querySelectorAll('[role="option"]')[1] as HTMLButtonElement).click();
    const input = picker.querySelector('input[type="text"]') as HTMLInputElement;
    expect(input).toBeTruthy();

    input.value = 'Not an identity';
    input.dispatchEvent(new Event('input'));
    expect(picker.querySelector('[role="alert"]')!.textContent).toBe('Invalid format');

    input.value = 'Other <other@example.com>';
    input.dispatchEvent(new Event('input'));
    expect(picker.querySelector('[role="alert"]')!.textContent).toBe('Unverified email');

    input.value = 'Custom Name <primary@example.com>';
    input.dispatchEvent(new Event('input'));
    expect((picker.querySelector('input[name="commit_name"]') as HTMLInputElement).value).toBe('Custom Name');
    expect((picker.querySelector('input[name="commit_email"]') as HTMLInputElement).value).toBe('primary@example.com');
    expect((settingsStore.get('git-identity:author:v3') as {identities: unknown[]}).identities[0]).toEqual({name: 'Custom Name', email: 'primary@example.com'});
  });

  test('reports email loading errors using the supplied translated message', async () => {
    mockedGet.mockRejectedValue(new Error('offline'));
    const picker = createPicker();
    await vi.waitFor(() => expect(picker.querySelector('[role="alert"]')!.textContent).toBe('Could not load emails'));
  });
});
