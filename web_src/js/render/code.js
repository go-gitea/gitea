import {getFileViewFilePath, getFileViewFileText, createExternalLink} from '../utils/misc.js';
import {basename, isObject} from '../utils.js';

export function initFileView() {
  if (document.querySelector('.file-view.code-view')) {
    const fileName = basename(getFileViewFilePath());
    if (fileName === 'package.json') {
      processPackageJson();
    }
  }
}

function processPackageJson() {
  let obj;
  try {
    obj = JSON.parse(getFileViewFileText());
  } catch {
    return;
  }
  if (!isObject(obj)) return;

  const packages = new Set();

  for (const key of [
    'dependencies',
    'dependenciesMeta',
    'devDependencies',
    'optionalDependencies',
    'overrides',
    'peerDependencies',
    'peerDependenciesMeta',
    'resolutions',
  ]) {
    for (const packageName of Object.keys(obj?.[key] || {})) {
      packages.add(packageName);
    }
  }

  // match chroma NameTag token to detect JSON object keys
  for (const el of document.querySelectorAll('.code-inner .nt')) {
    const jsonKey = el.textContent.replace(/^"(.*)"$/, '$1');
    if (packages.has(jsonKey)) {
      const link = createExternalLink({
        className: 'suppressed',
        textContent: jsonKey,
        href: `https://www.npmjs.com/package/${jsonKey}`,
      });
      el.textContent = '';
      el.append('"', link, '"');
    }
  }
}
