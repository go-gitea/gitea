export default async function createDropzone(el, opts) {
  import(/* webpackChunkName: "dropzone" */'dropzone/dist/dropzone.css');
  const {Dropzone} = await import(/* webpackChunkName: "dropzone" */'dropzone');
  Dropzone.autoDiscover = false;
  return new Dropzone(el, opts);
}
