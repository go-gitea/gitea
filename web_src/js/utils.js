export function svg(name, size) {
  return `<svg class="svg ${name}" width="${size}" height="${size}" aria-hidden="true"><use xlink:href="#${name}"/></svg>`;
}

// transform /path/to/file.ext to file.ext
export function basename(path = '') {
  return path ? path.replace(/^.*\//, '') : '';
}

// transform /path/to/file.ext to .ext
export function extname(path = '') {
  const [_, ext] = /.+(\.[^.]+)$/.exec(path) || [];
  return ext || '';
}
