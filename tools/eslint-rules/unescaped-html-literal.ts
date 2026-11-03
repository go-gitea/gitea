// MIT license, Copyright (c) GitHub, Inc.
// https://github.com/github/eslint-plugin-github/blob/main/lib/rules/unescaped-html-literal.js
import type {JSRuleDefinition, JSRuleDefinitionTypeOptions} from 'eslint';

const htmlTag = /<\/?[a-z][\w-]*[\s/>]/i;
const escapingTags = new Set(['html', 'htmlRaw']);

const rule: JSRuleDefinition<JSRuleDefinitionTypeOptions> = {
  meta: {
    type: 'problem',
    messages: {
      unescapedHtmlLiteral: 'Unescaped HTML literal. Use html`` tag template literal for secure escaping.',
    },
  },

  create: (context) => ({
    Literal(node) {
      if (typeof node.value !== 'string' || !htmlTag.test(node.value)) return;

      context.report({
        node,
        messageId: 'unescapedHtmlLiteral',
      });
    },
    TemplateLiteral(node) {
      if (node.quasis.every((quasi) => !htmlTag.test(quasi.value.raw))) return;

      const parent = node.parent;
      if (parent?.type === 'TaggedTemplateExpression' && parent.tag.type === 'Identifier' && escapingTags.has(parent.tag.name)) return;

      context.report({
        node,
        messageId: 'unescapedHtmlLiteral',
      });
    },
  }),
};

export default rule;
