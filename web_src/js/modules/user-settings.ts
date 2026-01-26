function getLocalStorageUserSetting(settingKey: string): string | null {
  const legacyKey = settingKey;
  const itemKey = `user-setting:${settingKey}`; // to avoid conflict with other localStorage items, use prefix
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
  } catch {}
  return null;
}

function setLocalStorageUserSetting(settingKey: string, value: string) {
  const legacyKey = settingKey;
  const itemKey = `user-setting:${settingKey}`;
  try {
    localStorage?.removeItem(legacyKey);
    localStorage?.setItem(itemKey, value);
  } catch {}
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
  getJsonObject: (key: string, def: any = null): any => {
    try {
      const value = getLocalStorageUserSetting(key);
      return value !== null ? JSON.parse(value) : def;
    } catch {}
    return def;
  },
  setJsonObject: (key: string, value: any) => {
    localUserSettings.setString(key, JSON.stringify(value));
  },
};

window.localUserSettings = localUserSettings;
