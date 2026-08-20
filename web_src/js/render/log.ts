import type {AnsiLineRenderer} from './ansi.ts';
import {createElementFromAttrs} from '../utils/dom.ts';

// How GitHub Actions logs work:
// * Workflow command outputs log commands like "::group::the-title", "::add-matcher::...."
// * Workflow runner parses and processes the commands to "##[group]", apply "matchers", hide secrets, etc.
// * The reported logs are the processed logs.
// HOWEVER: Gitea cannot, a decoded message may contain newlines and FormatLog drops them,
// so the commands arrive here still escaped and the frontend decodes them.
const LogLinePrefixCommandMap: Record<string, LogLineCommandName> = {
  '::group::': 'group',
  '##[group]': 'group',
  '::endgroup::': 'endgroup',
  '##[endgroup]': 'endgroup',
  '##[error]': 'error',
  '##[warning]': 'warning',
  '##[notice]': 'notice',
  '##[debug]': 'debug',
  '##[command]': 'command',
  '[command]': 'command',

  // https://github.com/actions/toolkit/blob/master/docs/commands.md
  // https://github.com/actions/runner/blob/main/docs/adrs/0276-problem-matchers.md#registration
  '::add-matcher::': 'hidden',
  '##[add-matcher]': 'hidden',
  '::remove-matcher': 'hidden', // it has arguments
};

// Pattern for ::cmd:: and ::cmd args:: format (args are stripped for display)
const LogLineCmdPattern = /^::(error|warning|notice|debug)(?:\s[^:]*)?::/;

export type LogLine = {
  index: number;
  timestamp: number;
  message: string;
};

export type LogLineCommandName = 'group' | 'endgroup' | 'command' | 'error' | 'warning' | 'notice' | 'debug' | 'hidden';
export type LogLineCommand = {
  name: LogLineCommandName;
  prefix: string;
};

export function parseLogLineCommand(line: LogLine): LogLineCommand | null {
  // TODO: in the future it can be refactored to be a general parser that can parse arguments, drop the "prefix match"
  for (const [prefix, commandName] of Object.entries(LogLinePrefixCommandMap)) {
    if (line.message.startsWith(prefix)) return {name: commandName, prefix};
  }
  // Handle ::cmd:: and ::cmd args:: format (runner may pass these through raw)
  const match = LogLineCmdPattern.exec(line.message);
  return match ? {name: match[1] as LogLineCommandName, prefix: match[0]} : null;
}

const LogLineLabelMap: Partial<Record<LogLineCommandName, string>> = {
  error: 'Error',
  warning: 'Warning',
  notice: 'Notice',
  debug: 'Debug',
};

export function decodeLogLineMessage(line: LogLine, command: LogLineCommand | null): string {
  // TODO: for some commands (::group::), the "prefix removal" works well, for some commands with "arguments" (::remove-matcher ...::),
  // it needs to do further processing in the future (fortunately, at the moment we don't need to handle these commands)
  if (!command) return line.message;
  let message = line.message.substring(command.prefix.length);
  if (command.name === 'command') return message; // "command" is only an output tag, do not parse or escape it
  // "##[cmd]" also escapes ";" and "]" which delimit its header, "::cmd::" does not
  if (!command.prefix.startsWith('::')) message = message.replace(/%3B/g, ';').replace(/%5D/g, ']');
  // AnsiLineRenderer breaks a line per "\r", so "%0D%0A" is one break. "%25" last keeps "%250A" literal
  return message.replace(/(?:%0D)?%0A/g, '\n').replace(/%0D/g, '\r').replace(/%25/g, '%');
}

export function createLogLineMessage(ansi: AnsiLineRenderer, line: LogLine, command: LogLineCommand | null) {
  const attrs = {class: 'log-msg'};
  if (command?.name) attrs.class += ` log-cmd-${command.name}`; // make it easier to add styles to some commands like "error"
  const content = decodeLogLineMessage(line, command);
  const element = createElementFromAttrs('span', attrs);
  const label = command ? LogLineLabelMap[command.name] : null;
  if (label) {
    element.append(createElementFromAttrs('span', {class: 'log-msg-label'}, `${label}:`));
    const message = document.createElement('span');
    ansi.renderLine(message, ` ${content.trimStart()}`);
    element.append(message);
  } else {
    ansi.renderLine(element, content);
  }
  return element;
}
