export function migrateLocalStorage() {
  if (!localStorage) return;

  let storedVersion;
  try {
    storedVersion = JSON.parse(localStorage.getItem('version'));
  } catch {
    return; // don't do any migration if version parsing fails
  }

  if (storedVersion < 1) { // migrate to v1
    localStorage.removeItem('markdown-editor-wiki');
    localStorage.removeItem('markdown-editor-default');
    localStorage.setItem('version', JSON.stringify(1));
  }
}
