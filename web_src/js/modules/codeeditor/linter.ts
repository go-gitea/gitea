import type {CodemirrorModules} from './main.ts';
import type {Extension} from '@codemirror/state';

/** Creates a linter for JSON files using `jsonParseLinter` from `@codemirror/lang-json`. */
export async function createJsonLinter(cm: CodemirrorModules): Promise<Extension> {
  const {jsonParseLinter} = await import('@codemirror/lang-json');
  const baseLinter = jsonParseLinter();
  return cm.lint.linter((view) => {
    return baseLinter(view).map((d) => {
      if (d.from === d.to) {
        const line = view.state.doc.lineAt(d.from);
        // expand to end of line content, or at least 1 char
        d.to = Math.min(Math.max(d.from + 1, line.to), view.state.doc.length);
      }
      return d;
    });
  });
}

/** Creates a generic linter that detects Lezer parse-tree error nodes. */
export function createSyntaxErrorLinter(cm: CodemirrorModules): Extension {
  return cm.lint.linter((view) => {
    const diagnostics: {from: number, to: number, severity: 'error', message: string}[] = [];
    const tree = cm.language.syntaxTree(view.state);
    tree.iterate({
      enter(node) {
        if (node.type.isError) {
          diagnostics.push({
            from: node.from,
            to: node.to === node.from ? Math.min(node.from + 1, view.state.doc.length) : node.to,
            severity: 'error',
            message: 'Syntax error',
          });
        }
      },
    });
    return diagnostics;
  });
}
