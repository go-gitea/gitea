// @ts-check
import {defineConfig} from 'stylelint-define-config';
import {fileURLToPath} from 'node:url';

const cssVarFiles = [
  fileURLToPath(new URL('web_src/css/base.css', import.meta.url)),
  fileURLToPath(new URL('web_src/css/themes/theme-gitea-light.css', import.meta.url)),
  fileURLToPath(new URL('web_src/css/themes/theme-gitea-dark.css', import.meta.url)),
];

export default defineConfig({
  extends: 'stylelint-config-recommended',
  plugins: [
    'stylelint-declaration-strict-value',
    'stylelint-declaration-block-no-ignored-properties',
    'stylelint-value-no-unknown-custom-properties',
    '@stylistic/stylelint-plugin',
  ],
  ignoreFiles: [
    '**/*.go',
    '/web_src/fomantic',
  ],
  overrides: [
    {
      files: ['**/chroma/*', '**/codemirror/*', '**/standalone/*', '**/console.css', 'font_i18n.css'],
      rules: {
        'scale-unlimited/declaration-strict-value': null,
      },
    },
    {
      files: ['**/chroma/*', '**/codemirror/*'],
      rules: {
        'block-no-empty': null,
      },
    },
    {
      files: ['**/*.vue'],
      customSyntax: 'postcss-html',
    },
  ],
  rules: {
    '@stylistic/at-rule-name-case': null,
    '@stylistic/at-rule-name-newline-after': null,
    '@stylistic/at-rule-name-space-after': null,
    '@stylistic/at-rule-semicolon-newline-after': null,
    '@stylistic/at-rule-semicolon-space-before': null,
    '@stylistic/block-closing-brace-empty-line-before': null,
    '@stylistic/block-closing-brace-newline-after': null,
    '@stylistic/block-closing-brace-newline-before': null,
    '@stylistic/block-closing-brace-space-after': null,
    '@stylistic/block-closing-brace-space-before': null,
    '@stylistic/block-opening-brace-newline-after': null,
    '@stylistic/block-opening-brace-newline-before': null,
    '@stylistic/block-opening-brace-space-after': null,
    '@stylistic/block-opening-brace-space-before': 'always',
    '@stylistic/color-hex-case': 'lower',
    '@stylistic/declaration-bang-space-after': 'never',
    '@stylistic/declaration-bang-space-before': null,
    '@stylistic/declaration-block-semicolon-newline-after': null,
    '@stylistic/declaration-block-semicolon-newline-before': null,
    '@stylistic/declaration-block-semicolon-space-after': null,
    '@stylistic/declaration-block-semicolon-space-before': 'never',
    '@stylistic/declaration-block-trailing-semicolon': null,
    '@stylistic/declaration-colon-newline-after': null,
    '@stylistic/declaration-colon-space-after': null,
    '@stylistic/declaration-colon-space-before': 'never',
    '@stylistic/function-comma-newline-after': null,
    '@stylistic/function-comma-newline-before': null,
    '@stylistic/function-comma-space-after': null,
    '@stylistic/function-comma-space-before': null,
    '@stylistic/function-max-empty-lines': 0,
    '@stylistic/function-parentheses-newline-inside': null,
    '@stylistic/function-parentheses-space-inside': null,
    '@stylistic/function-whitespace-after': null,
    '@stylistic/indentation': 2,
    '@stylistic/linebreaks': null,
    '@stylistic/max-empty-lines': 1,
    '@stylistic/max-line-length': null,
    '@stylistic/media-feature-colon-space-after': null,
    '@stylistic/media-feature-colon-space-before': 'never',
    '@stylistic/media-feature-name-case': null,
    '@stylistic/media-feature-parentheses-space-inside': null,
    '@stylistic/media-feature-range-operator-space-after': 'always',
    '@stylistic/media-feature-range-operator-space-before': 'always',
    '@stylistic/media-query-list-comma-newline-after': null,
    '@stylistic/media-query-list-comma-newline-before': null,
    '@stylistic/media-query-list-comma-space-after': null,
    '@stylistic/media-query-list-comma-space-before': null,
    '@stylistic/named-grid-areas-alignment': null,
    '@stylistic/no-empty-first-line': null,
    '@stylistic/no-eol-whitespace': true,
    '@stylistic/no-extra-semicolons': true,
    '@stylistic/no-missing-end-of-source-newline': null,
    '@stylistic/number-leading-zero': null,
    '@stylistic/number-no-trailing-zeros': null,
    '@stylistic/property-case': 'lower',
    '@stylistic/selector-attribute-brackets-space-inside': null,
    '@stylistic/selector-attribute-operator-space-after': null,
    '@stylistic/selector-attribute-operator-space-before': null,
    '@stylistic/selector-combinator-space-after': null,
    '@stylistic/selector-combinator-space-before': null,
    '@stylistic/selector-descendant-combinator-no-non-space': null,
    '@stylistic/selector-list-comma-newline-after': null,
    '@stylistic/selector-list-comma-newline-before': null,
    '@stylistic/selector-list-comma-space-after': 'always-single-line',
    '@stylistic/selector-list-comma-space-before': 'never-single-line',
    '@stylistic/selector-max-empty-lines': 0,
    '@stylistic/selector-pseudo-class-case': 'lower',
    '@stylistic/selector-pseudo-class-parentheses-space-inside': 'never',
    '@stylistic/selector-pseudo-element-case': 'lower',
    '@stylistic/string-quotes': 'double',
    '@stylistic/unicode-bom': null,
    '@stylistic/unit-case': 'lower',
    '@stylistic/value-list-comma-newline-after': null,
    '@stylistic/value-list-comma-newline-before': null,
    '@stylistic/value-list-comma-space-after': null,
    '@stylistic/value-list-comma-space-before': null,
    '@stylistic/value-list-max-empty-lines': 0,
    'at-rule-no-unknown': [true, {ignoreAtRules: ['tailwind']}],
    'at-rule-no-vendor-prefix': true,
    'csstools/value-no-unknown-custom-properties': [true, {importFrom: cssVarFiles}],
    'declaration-block-no-duplicate-properties': [true, {ignore: ['consecutive-duplicates-with-different-values']}],
    'declaration-block-no-redundant-longhand-properties': [true, {ignoreShorthands: ['flex-flow', 'overflow', 'grid-template']}],
    // @ts-expect-error - https://github.com/stylelint-types/stylelint-define-config/issues/1
    'declaration-property-unit-disallowed-list': {'line-height': ['em']},
    // @ts-expect-error - https://github.com/stylelint-types/stylelint-define-config/issues/1
    'declaration-property-value-disallowed-list': {'word-break': ['break-word']},
    'font-family-name-quotes': 'always-where-recommended',
    'function-name-case': 'lower',
    'function-url-quotes': 'always',
    'import-notation': 'string',
    'length-zero-no-unit': [true, {ignore: ['custom-properties'], ignoreFunctions: ['var']}],
    'media-feature-name-no-vendor-prefix': true,
    'no-descending-specificity': null,
    'no-invalid-position-at-import-rule': [true, {ignoreAtRules: ['tailwind']}],
    'no-unknown-animations': null, // disabled until stylelint supports multi-file linting
    'no-unknown-custom-media': null, // disabled until stylelint supports multi-file linting
    'no-unknown-custom-properties': null,  // disabled until stylelint supports multi-file linting
    'plugin/declaration-block-no-ignored-properties': true,
    'scale-unlimited/declaration-strict-value': [['/color$/', 'font-weight'], {ignoreValues: '/^(inherit|transparent|unset|initial|currentcolor|none)$/', ignoreFunctions: true, disableFix: true, expandShorthand: true}],
    'selector-attribute-quotes': 'always',
    'selector-no-vendor-prefix': true,
    'selector-pseudo-element-colon-notation': 'double',
    'selector-type-case': 'lower',
    'selector-type-no-unknown': [true, {ignore: ['custom-elements']}],
    'shorthand-property-no-redundant-values': true,
    'value-no-vendor-prefix': [true, {ignoreValues: ['box', 'inline-box']}],
  },
});
