export default {
  exclude: [
    'pretty-ms', // pretty-ms@9 requires BigInt literals which require esbuild target es2020 or newer
    '@mcaptcha/vanilla-glue', // broken when upgrading from alpha to rc
  ],
};
