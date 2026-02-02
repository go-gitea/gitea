// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

export type LogLine = {
  index: number;
  timestamp: number;
  message: string;
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
