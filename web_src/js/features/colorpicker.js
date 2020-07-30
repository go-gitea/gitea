export default async function createColorPicker($els) {
  if (!$els || !$els.length) return;

  await Promise.all([
    import(/* webpackChunkName: "minicolors" */'@claviska/jquery-minicolors'),
    import(/* webpackChunkName: "minicolors" */'@claviska/jquery-minicolors/jquery.minicolors.css'),
  ]);

  $els.minicolors();
}
