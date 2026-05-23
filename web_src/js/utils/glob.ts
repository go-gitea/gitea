// Reference: https://github.com/gobwas/glob/blob/master/glob.go
//
// Compile creates Glob for given pattern and strings (if any present after pattern) as separators.
// The pattern syntax is:
//
//    pattern:
//        { term }
//
//    term:
//        `*`         matches any sequence of non-separator characters
//        `**`        matches any sequence of characters
//        `?`         matches any single non-separator character
//        `[` [ `!` ] { character-range } `]`
//                    character class (must be non-empty)
//        `{` pattern-list `}`
//                    pattern alternatives
//        c           matches character c (c != `*`, `**`, `?`, `\`, `[`, `{`, `}`)
//        `\` c       matches character c
//
//    character-range:
//        c           matches character c (c != `\\`, `-`, `]`)
//        `\` c       matches character c
//        lo `-` hi   matches character c for lo <= c <= hi
//
//    pattern-list:
//        pattern { `,` pattern }
//                    comma-separated (without spaces) patterns
//

class GlobCompiler {
  nonSeparatorChars: string;
  globPattern: string;
  regexpPattern: string;
  regexp: RegExp;
  pos: number = 0;

  #compileChars(): string {
    let result = '';
    if (this.globPattern[this.pos] === '!') {
      this.pos++;
      result += '^';
    }
    while (this.pos < this.globPattern.length) {
      const c = this.globPattern[this.pos];
      this.pos++;
      if (c === ']') {
        return `[${result}]`;
      }
      if (c === '\\') {
        if (this.pos >= this.globPattern.length) {
          throw new Error('Unterminated character class escape');
        }
        this.pos++;
        result += `\\${this.globPattern[this.pos]}`;
      } else {
        result += c;
      }
    }
    throw new Error('Unterminated character class');
  }

  #compile(subPattern: boolean = false): string {
    let result = '';
    while (this.pos < this.globPattern.length) {
      const c = this.globPattern[this.pos];
      this.pos++;
      if (subPattern && c === '}') {
        return `(${result})`;
      }
      switch (c) {
        case '*':
          if (this.globPattern[this.pos] !== '*') {
            result += `${this.nonSeparatorChars}*`; // match any sequence of non-separator characters
          } else {
            this.pos++;
            result += '.*'; // match any sequence of characters
          }
          break;
        case '?':
          result += this.nonSeparatorChars; // match any single non-separator character
          break;
        case '[':
          result += this.#compileChars();
          break;
        case '{':
          result += this.#compile(true);
          break;
        case ',':
          result += subPattern ? '|' : ',';
          break;
        case '\\':
          if (this.pos >= this.globPattern.length) {
            throw new Error('No character to escape');
          }
          result += `\\${this.globPattern[this.pos]}`;
          this.pos++;
          break;
        case '.': case '+': case '^': case '$': case '(': case ')': case '|':
          result += `\\${c}`; // escape regexp special characters
          break;
        default:
          result += c;
      }
    }
    return result;
  }

  constructor(pattern: string, separators: string = '') {
    const escapedSeparators = separators.replaceAll(/[\^\]\-\\]/g, '\\$&');
    this.nonSeparatorChars = escapedSeparators ? `[^${escapedSeparators}]` : '.';
    this.globPattern = pattern;
    this.regexpPattern = `^${this.#compile()}$`;
    this.regexp = new RegExp(`^${this.regexpPattern}$`);
  }
}

export function globCompile(pattern: string, separators: string = ''): GlobCompiler {
  return new GlobCompiler(pattern, separators);
}

export function globMatch(str: string, pattern: string, separators: string = ''): boolean {
  try {
    return globCompile(pattern, separators).regexp.test(str);
  } catch {
    return false;
  }
}
