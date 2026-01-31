/* eslint-disable no-restricted-globals */
// Some people deploy Gitea under a subpath, so it needs prefix to avoid local storage key conflicts.
// And these keys are for user settings only, it also needs a specific prefix,
// in case in the future there are other uses of local storage, and/or we need to clear some keys when the quota is exceeded.
const itemKeyPrefix = 'gitea:setting:';

function handleLocalStorageError(e: any) {
  // in the future, maybe we need to handle quota exceeded errors differently
  console.error('Error using local storage for user settings', e);
}

function getLocalStorageUserSetting(settingKey: string): string | null {
  const legacyKey = settingKey;
  const itemKey = `${itemKeyPrefix}${settingKey}`;
  try {
    const legacyValue = localStorage?.getItem(legacyKey) ?? null;
    const value = localStorage?.getItem(itemKey) ?? null; // avoid undefined
    if (value !== null && legacyValue !== null) {
      // if both values exist, remove the legacy one
      localStorage?.removeItem(legacyKey);
    } else if (value === null && legacyValue !== null) {
      // migrate legacy value to new key
      localStorage?.removeItem(legacyKey);
      localStorage?.setItem(itemKey, legacyValue);
      return legacyValue;
    }
    return value;
  } catch (e) {
    handleLocalStorageError(e);
  }
  return null;
}

function setLocalStorageUserSetting(settingKey: string, value: string) {
  const legacyKey = settingKey;
  const itemKey = `${itemKeyPrefix}${settingKey}`;
  try {
    localStorage?.removeItem(legacyKey);
    localStorage?.setItem(itemKey, value);
  } catch (e) {
    handleLocalStorageError(e);
  }
}

export const localUserSettings = {
  getString: (key: string, def: string = ''): string => {
    return getLocalStorageUserSetting(key) ?? def;
  },
  setString: (key: string, value: string) => {
    setLocalStorageUserSetting(key, value);
  },
  getBoolean: (key: string, def: boolean = false): boolean => {
    return localUserSettings.getString(key, String(def)) === 'true';
  },
  setBoolean: (key: string, value: boolean) => {
    localUserSettings.setString(key, String(value));
  },
  getJsonObject: <T extends Record<string, any>>(key: string, def: T): T => {
    const value = getLocalStorageUserSetting(key);
    try {
      const decoded = value !== null ? JSON.parse(value) : def;
      return decoded ?? def;
    } catch (e) {
      console.error(`Unable to parse JSON value for local user settings ${key}=${value}`, e);
    }
    return def;
  },
  setJsonObject: <T extends Record<string, any>>(key: string, value: T) => {
    localUserSettings.setString(key, JSON.stringify(value));
  },
};

window.localUserSettings = localUserSettings;
