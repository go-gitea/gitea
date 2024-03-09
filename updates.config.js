export default {
  exclude: [
    '@mcaptcha/vanilla-glue', // breaking changes in rc versions
    'eslint-plugin-array-func', // need to migrate to eslint flat config first
    'monaco-editor', // https://github.com/microsoft/monaco-editor/issues/4325
  ],
};
