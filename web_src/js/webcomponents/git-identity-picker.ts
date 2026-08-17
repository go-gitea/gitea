import {GET} from '../modules/fetch.ts';
import {localUserSettings} from '../modules/user-settings.ts';
import {svg} from '../svg.ts';
import {createElementFromHTML} from '../utils/dom.ts';

type EmailRecord = {
  email: string;
  verified: boolean;
  primary: boolean;
};

type GitIdentity = {
  name: string;
  email: string;
};

type IdentityPreferences = {
  identities?: GitIdentity[];
};

const elementName = 'gitea-git-identity-picker';
const maxRecentIdentities = 5;
const identityPattern = /^(.+?)\s+<([^<>@\s]+@[^<>@\s]+)>$/;

function identityText(identity: GitIdentity) {
  return `${identity.name} <${identity.email}>`;
}

function sameIdentity(a: GitIdentity, b: GitIdentity) {
  return a.name === b.name && a.email === b.email;
}

export class GitIdentityPicker extends HTMLElement {
  private pickerRole!: string;
  private fieldPrefix!: string;
  private preferenceKey!: string;
  private identities: GitIdentity[] = [];
  private validEmails = new Set<string>();
  private currentIdentity!: GitIdentity;
  private customMode = false;
  private hiddenName!: HTMLInputElement;
  private hiddenEmail!: HTMLInputElement;
  private control!: HTMLElement;
  private customInput?: HTMLInputElement;
  private menu!: HTMLDivElement;
  private status!: HTMLSpanElement;

  connectedCallback() {
    if (this.hiddenName) return;
    this.pickerRole = this.getAttribute('data-role') === 'committer' ? 'committer' : 'author';
    this.fieldPrefix = this.getAttribute('data-field-prefix') || this.pickerRole;
    this.preferenceKey = `git-identity:${this.pickerRole}:v3`;
    this.currentIdentity = {
      name: this.getAttribute('data-default-name') || '',
      email: this.getAttribute('data-default-email') || '',
    };

    this.hiddenName = document.createElement('input');
    this.hiddenName.type = 'hidden';
    this.hiddenName.name = `${this.fieldPrefix}_name`;
    this.hiddenEmail = document.createElement('input');
    this.hiddenEmail.type = 'hidden';
    this.hiddenEmail.name = `${this.fieldPrefix}_email`;

    this.menu = document.createElement('div');
    this.menu.className = 'menu transition hidden';
    this.menu.id = `${elementName}-${this.pickerRole}-options`;
    this.menu.setAttribute('role', 'listbox');

    this.status = document.createElement('span');
    this.status.className = 'tw-ml-2 tw-text-red';
    this.status.setAttribute('role', 'alert');
    this.status.hidden = true;

    const dropdown = document.createElement('div');
    dropdown.className = 'ui dropdown custom branch-selector-dropdown ellipsis-text-items git-identity-picker-dropdown';
    dropdown.append(this.menu);
    this.replaceChildren(this.hiddenName, this.hiddenEmail, dropdown, this.status);
    this.renderPresetControl();
    this.updateSubmittedIdentity(this.currentIdentity);
    this.addEventListener('keydown', this.onKeyDown);
    document.addEventListener('mousedown', this.onDocumentMouseDown, {capture: true});
    this.closest('form')?.addEventListener('submit', this.onSubmit);
    this.loadEmails();
  }

  disconnectedCallback() {
    this.removeEventListener('keydown', this.onKeyDown);
    document.removeEventListener('mousedown', this.onDocumentMouseDown, {capture: true});
    this.closest('form')?.removeEventListener('submit', this.onSubmit);
  }

  private readonly onDocumentMouseDown = (event: MouseEvent) => {
    if (!this.contains(event.target as Node)) this.closeMenu();
  };

  private readonly onKeyDown = (event: KeyboardEvent) => {
    if (this.customMode) {
      if (event.key === 'Escape' && this.customInput?.value === '') {
        event.preventDefault();
        this.renderPresetControl();
      }
      return;
    }
    if (event.key === 'Escape') this.closeMenu();
    else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      this.toggleMenu();
    }
  };

  private readonly onSubmit = (event: Event) => {
    if (!this.customMode) return;
    if (this.applyCustomIdentity()) return;
    event.preventDefault();
    this.customInput?.focus();
  };

  private async loadEmails() {
    try {
      const response = await GET(`${window.config.appSubUrl}/user/emails`);
      if (!response.ok) throw new Error(`Request failed: ${response.status}`);
      const records = await response.json() as EmailRecord[];
      const emails = records
        .filter((record) => record.verified && typeof record.email === 'string')
        .sort((a, b) => Number(b.primary) - Number(a.primary));
      this.validEmails = new Set(emails.map((record) => record.email));
      this.identities = this.createIdentityOptions(emails);
      this.currentIdentity = this.resolveInitialIdentity();
      this.updateSubmittedIdentity(this.currentIdentity);
      this.renderPresetControl();
      this.status.hidden = true;
    } catch {
      this.showStatus(this.getAttribute('data-error-load') || '');
    }
  }

  private createIdentityOptions(emails: EmailRecord[]) {
    const defaults = emails.map((record) => ({
      name: this.getAttribute('data-default-name') || '',
      email: record.email,
    }));
    const saved = this.getRecentIdentities().filter((identity) => this.validEmails.has(identity.email));
    const all = [...saved, ...defaults];
    return all.filter((identity, index) => all.findIndex((other) => sameIdentity(other, identity)) === index);
  }

  private resolveInitialIdentity() {
    const saved = this.getRecentIdentities().find((identity) => this.validEmails.has(identity.email));
    if (saved) return saved;
    const defaultIdentity = {
      name: this.getAttribute('data-default-name') || '',
      email: this.getAttribute('data-default-email') || '',
    };
    if (this.validEmails.has(defaultIdentity.email)) return defaultIdentity;
    return this.identities[0] || this.currentIdentity;
  }

  private getRecentIdentities() {
    const preferences = localUserSettings.getJsonObject<IdentityPreferences>(this.preferenceKey, {});
    if (!Array.isArray(preferences.identities)) return [];
    return preferences.identities.filter((identity): identity is GitIdentity =>
      typeof identity?.name === 'string' && identity.name !== '' && typeof identity.email === 'string' && identity.email !== '',
    );
  }

  private saveIdentity(identity: GitIdentity) {
    const identities = [identity, ...this.getRecentIdentities().filter((other) => !sameIdentity(other, identity))]
      .slice(0, maxRecentIdentities);
    localUserSettings.setJsonObject(this.preferenceKey, {identities});
  }

  private updateSubmittedIdentity(identity: GitIdentity) {
    this.currentIdentity = identity;
    this.hiddenName.value = identity.name;
    this.hiddenEmail.value = identity.email;
    this.dispatchEvent(new CustomEvent('git-identity-change', {detail: identity, bubbles: true}));
  }

  private renderPresetControl() {
    this.customMode = false;
    this.closeMenu();
    const button = document.createElement('button');
    button.type = 'button';
    button.id = `${elementName}-${this.pickerRole}`;
    button.className = 'ui button branch-dropdown-button git-identity-picker-button tw-flex tw-items-center tw-justify-between';
    button.setAttribute('aria-haspopup', 'listbox');
    button.setAttribute('aria-expanded', 'false');
    button.setAttribute('aria-controls', this.menu.id);
    const text = document.createElement('span');
    text.className = 'gt-ellipsis';
    text.textContent = identityText(this.currentIdentity);
    const icon = createElementFromHTML<SVGElement>(svg('octicon-triangle-down', 14, 'dropdown icon'));
    button.append(text, icon);
    button.addEventListener('click', () => this.toggleMenu());
    this.control?.replaceWith(button);
    this.control = button;
    this.menu.before(button);
    this.renderMenu();
  }

  private renderMenu() {
    this.menu.replaceChildren();
    for (const identity of this.identities) {
      const item = document.createElement('button');
      item.type = 'button';
      item.className = `item${sameIdentity(identity, this.currentIdentity) ? ' selected' : ''}`;
      item.textContent = identityText(identity);
      item.setAttribute('role', 'option');
      item.setAttribute('aria-selected', String(sameIdentity(identity, this.currentIdentity)));
      item.addEventListener('click', () => {
        this.updateSubmittedIdentity(identity);
        this.saveIdentity(identity);
        this.renderPresetControl();
      });
      this.menu.append(item);
    }
    const custom = document.createElement('button');
    custom.type = 'button';
    custom.className = 'item';
    custom.textContent = this.getAttribute('data-custom-label') || '';
    custom.setAttribute('role', 'option');
    custom.addEventListener('click', () => this.renderCustomControl());
    this.menu.append(custom);
  }

  private renderCustomControl() {
    this.customMode = true;
    this.closeMenu();
    const input = document.createElement('input');
    input.type = 'text';
    input.id = `${elementName}-${this.pickerRole}`;
    input.className = 'git-identity-picker-input';
    input.value = identityText(this.currentIdentity);
    input.placeholder = this.getAttribute('data-placeholder') || '';
    input.autocomplete = 'off';
    input.setAttribute('aria-invalid', 'false');
    input.addEventListener('input', () => this.applyCustomIdentity());
    input.addEventListener('blur', () => this.applyCustomIdentity());
    const restore = document.createElement('button');
    restore.type = 'button';
    restore.className = 'ui compact button git-identity-picker-restore';
    restore.setAttribute('aria-label', this.getAttribute('data-restore-label') || '');
    restore.setAttribute('title', this.getAttribute('data-restore-label') || '');
    restore.append(createElementFromHTML<SVGElement>(svg('octicon-x', 14)));
    restore.addEventListener('click', () => this.renderPresetControl());
    const controlGroup = document.createElement('div');
    controlGroup.className = 'ui fluid action input git-identity-picker-custom';
    controlGroup.append(input, restore);
    this.control.replaceWith(controlGroup);
    this.control = controlGroup;
    this.customInput = input;
    input.focus();
    input.select();
  }

  private applyCustomIdentity() {
    const input = this.customInput!;
    const match = identityPattern.exec(input.value.trim());
    if (!match) {
      this.showStatus(this.getAttribute('data-error-format') || '');
      input.setAttribute('aria-invalid', 'true');
      return false;
    }
    const identity = {name: match[1].trim(), email: match[2]};
    if (!this.validEmails.has(identity.email)) {
      this.showStatus(this.getAttribute('data-error-unverified') || '');
      input.setAttribute('aria-invalid', 'true');
      return false;
    }
    input.setAttribute('aria-invalid', 'false');
    this.status.hidden = true;
    this.updateSubmittedIdentity(identity);
    this.saveIdentity(identity);
    return true;
  }

  private showStatus(message: string) {
    this.status.textContent = message;
    this.status.hidden = !message;
  }

  private toggleMenu() {
    if (this.menu.classList.contains('hidden')) this.openMenu();
    else this.closeMenu();
  }

  private openMenu() {
    this.menu.classList.replace('hidden', 'visible');
    this.control.setAttribute('aria-expanded', 'true');
  }

  private closeMenu() {
    this.menu.classList.replace('visible', 'hidden');
    this.control?.setAttribute('aria-expanded', 'false');
  }
}

if (!window.customElements.get(elementName)) window.customElements.define(elementName, GitIdentityPicker);
