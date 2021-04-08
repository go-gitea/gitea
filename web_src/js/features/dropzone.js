export default async function createDropzone(el, opts) {
  const [{default: Dropzone}] = await Promise.all([
    import(/* webpackChunkName: "dropzone" */'dropzone'),
    import(/* webpackChunkName: "dropzone" */'dropzone/dist/dropzone.css'),
  ]);

  Dropzone.Dropzone.autoDiscover = false;
  return new Dropzone.Dropzone(el, opts);
}
