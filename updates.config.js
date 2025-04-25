export default {
  exclude: [
    '@mcaptcha/vanilla-glue', // breaking changes in rc versions need to be handled
    '@stylistic/eslint-plugin-js', // need to migrate to eslint 9
    'cropperjs', // need to migrate to v2 but v2 is not compatible with v1
    'eslint', // need to migrate to eslint flat config first
    'eslint-plugin-array-func', // need to migrate to eslint flat config first
    'eslint-plugin-github', // need to migrate to eslint 9 - https://github.com/github/eslint-plugin-github/issues/585
    'eslint-plugin-no-use-extend-native', // need to migrate to eslint flat config first
    'eslint-plugin-unicorn', // need to migrate to eslint 9
    'eslint-plugin-vitest', // need to migrate to eslint flat config first
    'tailwindcss', // need to migrate
  ],
};
