import $ from 'jquery';

export async function createColorPicker(els) {
  if (!els.length) return;

  await Promise.all([
    import(/* webpackChunkName: "minicolors" */'@claviska/jquery-minicolors'),
    import(/* webpackChunkName: "minicolors" */'@claviska/jquery-minicolors/jquery.minicolors.css'),
  ]);

  return $(els).minicolors();
}
