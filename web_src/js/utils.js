export function svg(name, size) {
  return `<svg class="svg ${name}" width="${size}" height="${size}" aria-hidden="true"><use xlink:href="#${name}"/></svg>`;
}

export function basename(path = '') {
  if (!path) return path;
  return path.replace(/^.*\//, '');
}

export function extname(path = '') {
  const [_, ext] = /.+\.([^.]+)$/.exec(path) || [];
  return ext || '';
}
