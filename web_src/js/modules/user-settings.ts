function getLocalStorageUserSetting(settingKey: string): string | null {
  const legacyKey = settingKey;
  const itemKey = `user-setting:${settingKey}`;
  try {
    const legacyValue = localStorage?.getItem(legacyKey);
    const value = localStorage?.getItem(itemKey);
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
  } catch {
    return null;
  }
}

function setLocalStorageUserSetting(settingKey: string, value: string) {
  const legacyKey = settingKey;
  const itemKey = `user-setting:${settingKey}`;
  localStorage?.removeItem(legacyKey);
  localStorage?.setItem(itemKey, value);
}

export const localUserSettings = {
  getString: (key: string, def: string = ''): string => {
    const value = getLocalStorageUserSetting(key);
    return value !== null ? value : def;
  },
  setString: (key: string, value: string) => {
    setLocalStorageUserSetting(key, value);
  },
  getBoolean: (key: string, def: boolean = false):boolean => {
    return localUserSettings.getString(key, String(def)) === 'true';
  },
  setBoolean: (key: string, value: boolean) => {
    localUserSettings.setString(key, String(value));
  },
  getJsonObject: (key: string, def: any = null):any => {
    try {
      const value = getLocalStorageUserSetting(key);
      if (value === null) return def;
      return JSON.parse(value);
    } catch {
      return def;
    }
  },
  setJsonObject: (key: string, value: any) => {
    localUserSettings.setString(key, JSON.stringify(value));
  },
};

window.localUserSettings = localUserSettings;
