export function svg(name, size) {
  return `<svg class="svg ${name}" width="${size}" height="${size}" aria-hidden="true"><use xlink:href="${window.config.StaticPrefix}/img/svg/icons.svg#${name}"/></svg>`;
}
