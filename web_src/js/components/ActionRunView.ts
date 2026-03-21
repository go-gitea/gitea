import {createElementFromAttrs} from '../utils/dom.ts';
import {renderAnsi} from '../render/ansi.ts';

// How GitHub Actions logs work:
// * Workflow command outputs log commands like "::group::the-title", "::add-matcher::...."
// * Workflow runner parses and processes the commands to "##[group]", apply "matchers", hide secrets, etc.
// * The reported logs are the processed logs.
// HOWEVER: Gitea runner does not completely process those commands. Many works are done by the frontend at the moment.
const LogLinePrefixCommandMap: Record<string, LogLineCommandName> = {
  '::group::': 'group',
  '##[group]': 'group',
  '::endgroup::': 'endgroup',
  '##[endgroup]': 'endgroup',

  '##[error]': 'error',
  '[command]': 'command',

  // https://github.com/actions/toolkit/blob/master/docs/commands.md
  // https://github.com/actions/runner/blob/main/docs/adrs/0276-problem-matchers.md#registration
  '::add-matcher::': 'hidden',
  '##[add-matcher]': 'hidden',
  '::remove-matcher': 'hidden', // it has arguments
};

export type LogLine = {
  index: number;
  timestamp: number;
  message: string;
};

export type LogLineCommandName = 'group' | 'endgroup' | 'command' | 'error' | 'hidden';
export type LogLineCommand = {
  name: LogLineCommandName,
  prefix: string,
};

export function parseLogLineCommand(line: LogLine): LogLineCommand | null {
  // TODO: in the future it can be refactored to be a general parser that can parse arguments, drop the "prefix match"
  for (const prefix of Object.keys(LogLinePrefixCommandMap)) {
    if (line.message.startsWith(prefix)) {
      return {name: LogLinePrefixCommandMap[prefix], prefix};
    }
  }
  return null;
}

export function createLogLineMessage(line: LogLine, cmd: LogLineCommand | null) {
  const logMsgAttrs = {class: 'log-msg'};
  if (cmd?.name) logMsgAttrs.class += ` log-cmd-${cmd?.name}`; // make it easier to add styles to some commands like "error"

  // TODO: for some commands (::group::), the "prefix removal" works well, for some commands with "arguments" (::remove-matcher ...::),
  // it needs to do further processing in the future (fortunately, at the moment we don't need to handle these commands)
  const msgContent = cmd ? line.message.substring(cmd.prefix.length) : line.message;

  const logMsg = createElementFromAttrs('span', logMsgAttrs);
  logMsg.innerHTML = renderAnsi(msgContent);
  return logMsg;
}
