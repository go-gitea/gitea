const { StaticUrlPrefix } = window.config;

export function svg(name, size) {
  return `<svg class="svg ${name}" width="${size}" height="${size}" aria-hidden="true"><use xlink:href="${StaticUrlPrefix}/img/svg/icons.svg#${name}"/></svg>`;
}
