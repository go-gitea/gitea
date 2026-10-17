import {GET} from '../fetch.ts';

type DiffStreamLFSObject = {size: number; exists: boolean};

export type DiffStreamFile = {
  name: string;
  oldName: string;
  nameHash: string;
  type: 'add' | 'change' | 'delete' | 'rename' | 'copy';
  oldMode?: string;
  mode?: string;
  isBinary?: boolean;
  isVendored?: boolean;
  isGenerated?: boolean;
  isProtected?: boolean;
  language?: string;
  tabSize: number;
  submodule?: {repoLink?: string; oldCommit?: string; newCommit?: string; oldLink?: string; newLink?: string; compareLink?: string};
  lfs?: {old?: DiffStreamLFSObject; new?: DiffStreamLFSObject};
  oldSize?: number;
  newSize?: number;
  oldImage?: string; // the mime type if it's an image
  newImage?: string;
  tooLarge?: boolean;
  hunks?: string;
};

type DiffStreamEnd = {end: true; truncated: boolean; failed: boolean};

// reads the newline-delimited JSON written by the server's diff stream, onFile is called as soon as a file arrives
export async function streamDiff(url: string, onFile: (file: DiffStreamFile) => void): Promise<DiffStreamEnd> {
  const response = await GET(url);
  if (!response.ok) throw new Error(`Failed to load the diff: ${response.status}`);
  const reader = response.body!.pipeThrough(new TextDecoderStream()).getReader();
  let pending = '';
  let end: DiffStreamEnd = {end: true, truncated: false, failed: true}; // a missing end line means the stream broke
  const handleLine = (line: string) => {
    if (!line) return;
    const item = JSON.parse(line);
    if (item.end) end = item;
    else onFile(item);
  };
  for (let chunk = await reader.read(); !chunk.done; chunk = await reader.read()) {
    const text = chunk.value;
    let start = 0;
    for (let newline = text.indexOf('\n'); newline !== -1; newline = text.indexOf('\n', start)) {
      handleLine(pending + text.slice(start, newline));
      pending = '';
      start = newline + 1;
    }
    pending += text.slice(start);
  }
  handleLine(pending);
  return end;
}
