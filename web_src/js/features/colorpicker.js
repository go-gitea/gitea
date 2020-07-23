export default async function createColorPicker($els) {
  await Promise.all([
    import(/* webpackChunkName: "minicolors" */'@claviska/jquery-minicolors'),
    import(/* webpackChunkName: "minicolors" */'@claviska/jquery-minicolors/jquery.minicolors.css'),
  ]);

  $els.minicolors();
}
