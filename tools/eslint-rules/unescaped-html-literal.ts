// MIT license, Copyright (c) GitHub, Inc.
// https://github.com/github/eslint-plugin-github/blob/main/lib/rules/unescaped-html-literal.js
import type {JSRuleDefinition, JSRuleDefinitionTypeOptions} from 'eslint';

const htmlOpenTag = /^\s*<[a-zA-Z]/;

const rule: JSRuleDefinition<JSRuleDefinitionTypeOptions> = {
  meta: {
    type: 'problem',
    messages: {
      unescapedHtmlLiteral: 'Unescaped HTML literal. Use html`` tag template literal for secure escaping.',
    },
  },

  create: (context) => ({
    Literal(node) {
      if (typeof node.value !== 'string' || !htmlOpenTag.test(node.value)) return;

      context.report({
        node,
        messageId: 'unescapedHtmlLiteral',
      });
    },
    TemplateLiteral(node) {
      const templateStart = node.quasis[0]?.value.raw;
      if (!templateStart || !htmlOpenTag.test(templateStart)) return;

      const parent = node.parent;
      if (parent?.type === 'TaggedTemplateExpression' && parent.tag.type === 'Identifier' && parent.tag.name === 'html') return;

      context.report({
        node,
        messageId: 'unescapedHtmlLiteral',
      });
    },
  }),
};

export default rule;
