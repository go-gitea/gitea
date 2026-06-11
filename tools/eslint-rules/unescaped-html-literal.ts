import type {JSRuleDefinition as RuleDefinition, JSRuleDefinitionTypeOptions as RuleDefinitionTypeOptions} from 'eslint';

type UnescapedHtmlLiteralRuleDefinitionTypeOptions = RuleDefinitionTypeOptions & {
  MessageIds: 'unescapedHtmlLiteral';
  RuleOptions: [];
};

const htmlOpenTag = /^\s*<[a-zA-Z]/;

const rule: RuleDefinition<UnescapedHtmlLiteralRuleDefinitionTypeOptions> = {
  meta: {
    type: 'problem',
    docs: {
      description: 'disallow unescaped HTML literals',
      url: 'https://github.com/go-gitea/gitea/blob/main/tools/eslint-rules/unescaped-html-literal.ts',
    },
    schema: [],
    messages: {
      unescapedHtmlLiteral: 'Unescaped HTML literal. Use html`` tag template literal for secure escaping.',
    },
  },

  create(context) {
    return {
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
    };
  },
};

export default rule;
