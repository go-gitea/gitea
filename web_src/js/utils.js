export function svg(name, size, staticPrefix) {
  return `<svg class="svg ${name}" width="${size}" height="${size}" aria-hidden="true"><use xlink:href="${staticPrefix}/img/svg/icons.svg#${name}"/></svg>`;
}
