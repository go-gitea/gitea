export function svg(name, size) {
  return `<svg class="svg ${name}" width="${size}" height="${size}" aria-hidden="true"><use xlink:href="#${name}"/></svg>`;
}
