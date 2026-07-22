// MIT license, Copyright (c) GitHub, Inc.
// https://github.com/github/eslint-plugin-github/blob/main/lib/rules/unescaped-html-literal.js
/* eslint-disable no-template-curly-in-string */
import rule from './unescaped-html-literal.ts';
import {RuleTester} from 'eslint';

class VitestRuleTester extends RuleTester {
  static describe = describe;
  static it = it;
  static itOnly = it.only;
}

const ruleTester = new VitestRuleTester();

ruleTester.run('unescaped-html-literal', rule, {
  valid: [
    {
      code: '`Hello World!`;',
      languageOptions: {ecmaVersion: 2017},
    },
    {
      code: "'Hello World!'",
      languageOptions: {ecmaVersion: 2017},
    },
    {
      code: '"Hello World!"',
      languageOptions: {ecmaVersion: 2017},
    },
    {
      code: 'const helloTemplate = () => html`<div>Hello World!</div>`;',
      languageOptions: {ecmaVersion: 2017},
    },
    {
      code: 'const helloTemplate = (name) => html`<div>Hello ${name}!</div>`;',
      languageOptions: {ecmaVersion: 2017},
    },
  ],
  invalid: [
    {
      code: "const helloHTML = '<div>Hello, World!</div>'",
      languageOptions: {ecmaVersion: 2017},
      errors: [
        {
          message: 'Unescaped HTML literal. Use html`` tag template literal for secure escaping.',
        },
      ],
    },
    {
      code: 'const helloHTML = "<h1>Hello, World!</h1>"',
      languageOptions: {ecmaVersion: 2017},
      errors: [
        {
          message: 'Unescaped HTML literal. Use html`` tag template literal for secure escaping.',
        },
      ],
    },
    {
      code: 'const helloHTML = `<div>Hello ${name}!</div>`',
      languageOptions: {ecmaVersion: 2017},
      errors: [
        {
          message: 'Unescaped HTML literal. Use html`` tag template literal for secure escaping.',
        },
      ],
    },
    {
      code: 'const helloHTML = ` \n\t<div>Hello ${name}!</div>`',
      languageOptions: {ecmaVersion: 2017},
      errors: [
        {
          message: 'Unescaped HTML literal. Use html`` tag template literal for secure escaping.',
        },
      ],
    },
    {
      code: 'const helloHTML = foo`<div>Hello ${name}!</div>`',
      languageOptions: {ecmaVersion: 2017},
      errors: [
        {
          message: 'Unescaped HTML literal. Use html`` tag template literal for secure escaping.',
        },
      ],
    },
  ],
});
