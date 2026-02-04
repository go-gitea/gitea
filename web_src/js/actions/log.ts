export type LogLine = {
  index: number; // 1
  message: string; // "message"
  timestamp: number; // 1770061591.330781
};

export const LogLinePrefixesGroup = ['::group::', '##[group]'];
export const LogLinePrefixesEndGroup = ['::endgroup::', '##[endgroup]'];
export const LogLinePrefixesHidden = ['::add-matcher::', '##[add-matcher]', '::remove-matcher'];

export type LogLineCommand = {
  name: 'group' | 'endgroup';
  prefix: string;
};

export function parseLineCommand(line: LogLine): LogLineCommand | null {
  for (const prefix of LogLinePrefixesGroup) {
    if (line.message.startsWith(prefix)) {
      return {name: 'group', prefix};
    }
  }
  for (const prefix of LogLinePrefixesEndGroup) {
    if (line.message.startsWith(prefix)) {
      return {name: 'endgroup', prefix};
    }
  }
  return null;
}

export function shouldHideLine(line: LogLine): boolean {
  for (const prefix of LogLinePrefixesHidden) {
    if (line.message.startsWith(prefix)) {
      return true;
    }
  }
  return false;
}
