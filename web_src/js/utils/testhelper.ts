// there could be different "testing" concepts, for example: backend's "setting.IsInTesting"
// even if backend is in testing mode, frontend could be complied in production mode
// so this function only checks if the frontend is in unit testing mode (usually from *.test.ts files)
export function isInFrontendUnitTest() {
  return import.meta.env.TEST === 'true';
}

/** strip common indentation from a string and trim it */
export function dedent(str: string) {
  const match = str.match(/^[ \t]*(?=\S)/gm);
  if (!match) return str;

  let minIndent = Number.POSITIVE_INFINITY;
  for (const indent of match) {
    minIndent = Math.min(minIndent, indent.length);
  }
  if (minIndent === 0 || minIndent === Number.POSITIVE_INFINITY) {
    return str;
  }

  return str.replace(new RegExp(`^[ \\t]{${minIndent}}`, 'gm'), '').trim();
}
