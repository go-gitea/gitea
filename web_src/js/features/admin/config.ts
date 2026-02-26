import {showTemporaryTooltip} from '../../modules/tippy.ts';
import {POST} from '../../modules/fetch.ts';
import {registerGlobalInitFunc} from '../../modules/observer.ts';
import {queryElems} from '../../utils/dom.ts';
import {submitFormFetchAction} from '../common-fetch-action.ts';

const {appSubUrl} = window.config;

function initSystemConfigAutoCheckbox(el: HTMLInputElement) {
  el.addEventListener('change', async () => {
    // if the checkbox is inside a form, we assume it's handled by the form submit and do not send an individual request
    if (el.closest('form')) return;
    try {
      const resp = await POST(`${appSubUrl}/-/admin/config`, {
        data: new URLSearchParams({key: el.getAttribute('data-config-dyn-key')!, value: String(el.checked)}),
      });
      const json: Record<string, any> = await resp.json();
      if (json.errorMessage) throw new Error(json.errorMessage);
    } catch (ex) {
      showTemporaryTooltip(el, ex.toString());
      el.checked = !el.checked;
    }
  });
}

type GeneralFormFieldElement = HTMLInputElement;

function unsupportedElement(el: Element): never {
  // HINT: for future developers: if you need to handle a config that cannot be directly mapped to a form element, you should either:
  // * Add a "hidden" input to store the value (not configurable)
  // * Design a new "component" to handle the config
  throw new Error(`Unsupported config form value mapping for ${el.nodeName} (name=${(el as HTMLInputElement).name},type=${(el as HTMLInputElement).type}), please add more and design carefully`);
}

function requireExplicitValueType(el: Element): never {
  throw new Error(`Unsupported config form value type for ${el.nodeName} (name=${(el as HTMLInputElement).name},type=${(el as HTMLInputElement).type}), please add explicit value type with "data-config-value-type" attribute`);
}

// try to extract the subKey for the config value from the element name
// * return '' if the element name exactly matches the config key, which means the value is directly stored in the element
// * return null if the config key not match
function extractElemConfigSubKey(el: GeneralFormFieldElement, dynKey: string): string | null {
  if (el.name === dynKey) return '';
  if (el.name.startsWith(`${dynKey}.`)) return el.name.slice(dynKey.length + 1); // +1 for the dot
  return null;
}

// Due to the different design between HTML form elements and the JSON struct of the config values, we need to explicitly define some types.
// * checkbox can be used for boolean value, it can also be used for multiple values (array)
type ConfigValueType = 'boolean' | 'string' | 'number' | 'timestamp'; // TODO: support more types like array, not used at the moment.

function toDatetimeLocalValue(unixSeconds: number) {
  const d = new Date(unixSeconds * 1000);
  return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
}

export class ConfigFormValueMapper {
  form: HTMLFormElement;
  presetJsonValues: Record<string, any> = {};
  presetValueTypes: Record<string, ConfigValueType> = {};

  constructor(form: HTMLFormElement) {
    this.form = form;
    for (const el of queryElems<HTMLInputElement>(form, '[data-config-value-json]')) {
      const dynKey = el.getAttribute('data-config-dyn-key')!;
      const jsonStr = el.getAttribute('data-config-value-json');
      try {
        this.presetJsonValues[dynKey] = JSON.parse(jsonStr || '{}'); // empty string also is valid, default to an empty object
      } catch (error) {
        this.presetJsonValues[dynKey] = {}; // in case the value in database is corrupted, don't break the whole form
        console.error(`Error parsing JSON for config ${dynKey}:`, error);
      }
    }
    for (const el of queryElems<HTMLInputElement>(form, '[data-config-value-type]')) {
      const valKey = el.getAttribute('data-config-dyn-key') || el.name;
      this.presetValueTypes[valKey] = el.getAttribute('data-config-value-type')! as ConfigValueType;
    }
  }

  // try to assign the config value to the form element, return true if assigned successfully,
  // otherwise return false (e.g. the element is not related to the config key)
  assignConfigValueToFormElement(el: GeneralFormFieldElement, dynKey: string, cfgVal: any) {
    const subKey = extractElemConfigSubKey(el, dynKey);
    if (subKey === null) return false; // if not match, skip

    const val = subKey ? cfgVal![subKey] : cfgVal;
    if (val === null) return true; // if name matches, but no value to assign, also succeed because the form element does exist
    const valType = this.presetValueTypes[el.name];
    if (el.matches('[type="checkbox"]')) {
      if (valType !== 'boolean') requireExplicitValueType(el);
      el.checked = Boolean(val ?? el.checked);
    } else if (el.matches('[type="datetime-local"]')) {
      if (valType !== 'timestamp') requireExplicitValueType(el);
      if (val) el.value = toDatetimeLocalValue(val);
    } else if (el.matches('textarea')) {
      el.value = String(val ?? el.value);
    } else if (el.matches('input') && (el.getAttribute('type') ?? 'text') === 'text') {
      el.value = String(val ?? el.value);
    } else {
      unsupportedElement(el);
    }
    return true;
  }

  collectConfigValueFromElement(el: GeneralFormFieldElement, _oldVal: any = null) {
    let val: any;
    const valType = this.presetValueTypes[el.name];
    if (el.matches('[type="checkbox"]')) {
      if (valType !== 'boolean') requireExplicitValueType(el);
      val = el.checked;
      // oldVal: for future use when we support array value with checkbox
    } else if (el.matches('[type="datetime-local"]')) {
      if (valType !== 'timestamp') requireExplicitValueType(el);
      val = Math.floor(new Date(el.value).getTime() / 1000) ?? 0; // NaN is fine to JSON.stringify, it becomes null.
    } else if (el.matches('textarea')) {
      val = el.value;
    } else if (el.matches('input') && (el.getAttribute('type') ?? 'text') === 'text') {
      val = el.value;
    } else {
      unsupportedElement(el);
    }
    return val;
  }

  collectConfigSubValues(namedElems: Array<GeneralFormFieldElement | null>, dynKey: string, cfgVal: Record<string, any>) {
    for (let idx = 0; idx < namedElems.length; idx++) {
      const el = namedElems[idx];
      if (!el) continue;
      const subKey = extractElemConfigSubKey(el, dynKey);
      if (!subKey) continue; // if not match, skip
      cfgVal[subKey] = this.collectConfigValueFromElement(el, cfgVal[subKey]);
      namedElems[idx] = null;
    }
  }

  fillFromSystemConfig() {
    for (const [dynKey, cfgVal] of Object.entries(this.presetJsonValues)) {
      const elems = this.form.querySelectorAll<GeneralFormFieldElement>(`[name^="${CSS.escape(dynKey)}"]`);
      let assigned = false;
      for (const el of elems) {
        if (this.assignConfigValueToFormElement(el, dynKey, cfgVal)) {
          assigned = true;
        }
      }
      if (!assigned) throw new Error(`Could not find form element for config ${dynKey}, please check the form design and json struct`);
    }
  }

  // TODO: OPEN-WITH-EDITOR-APP-JSON: need to use the same logic as backend
  marshalConfigValueOpenWithEditorApps(cfgVal: string): string {
    const apps: Array<{DisplayName: string, OpenURL: string}> = [];
    const lines = cfgVal.split('\n');
    for (const line of lines) {
      let [displayName, openUrl] = line.split('=', 2);
      displayName = displayName.trim();
      openUrl = openUrl?.trim() ?? '';
      if (!displayName || !openUrl) continue;
      apps.push({DisplayName: displayName, OpenURL: openUrl});
    }
    return JSON.stringify(apps);
  }

  marshalConfigValue(dynKey: string, cfgVal: any): string {
    if (dynKey === 'repository.open-with.editor-apps') return this.marshalConfigValueOpenWithEditorApps(cfgVal);
    return JSON.stringify(cfgVal);
  }

  collectToFormData(): FormData {
    const namedElems: Array<GeneralFormFieldElement | null> = [];
    queryElems(this.form, '[name]', (el) => namedElems.push(el as GeneralFormFieldElement));

    // first, process the config options with sub values, for example:
    // merge "foo.bar.Enabled", "foo.bar.Message" to "foo.bar"
    const formData = new FormData();
    for (const [dynKey, cfgVal] of Object.entries(this.presetJsonValues)) {
      this.collectConfigSubValues(namedElems, dynKey, cfgVal);
      formData.append('key', dynKey);
      formData.append('value', this.marshalConfigValue(dynKey, cfgVal));
    }

    // now, the namedElems should only contain the config options without sub values,
    // directly store the value in formData with key as the element name, for example:
    for (const el of namedElems) {
      if (!el) continue;
      const dynKey = el.name;
      const newVal = this.collectConfigValueFromElement(el);
      formData.append('key', dynKey);
      formData.append('value', this.marshalConfigValue(dynKey, newVal));
    }
    return formData;
  }
}

function initSystemConfigForm(form: HTMLFormElement) {
  const formMapper = new ConfigFormValueMapper(form);
  formMapper.fillFromSystemConfig();
  form.addEventListener('submit', async (e) => {
    if (!form.reportValidity()) return;
    e.preventDefault();
    const formData = formMapper.collectToFormData();
    await submitFormFetchAction(form, {formData});
  });
}

export function initAdminConfigs(): void {
  registerGlobalInitFunc('initAdminConfigSettings', (el) => {
    queryElems(el, 'input[type="checkbox"][data-config-dyn-key]', initSystemConfigAutoCheckbox);
    queryElems(el, 'form.system-config-form', initSystemConfigForm);
  });
}
