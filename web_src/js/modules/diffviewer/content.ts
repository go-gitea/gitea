import {pathEscapeSegments} from '../../utils/url.ts';
import {splitLines, type DiffHunk, type DiffSide} from './parse.ts';
import {fetchOk, streamDiff, type DiffStreamFile} from './stream.ts';
import {buildHunks} from './linediff.ts';
import type {FileContent} from './entry.ts';
import type {DiffViewerConfig} from './types.ts';

const lfsSniffLength = 8000; // like git, content with a NUL byte in its first bytes is binary

// a file of the commit before ("old") or after ("new") the changes, the "src" pages need a repo link
export function fileUrl(config: DiffViewerConfig, view: 'src' | 'raw' | 'media', side: DiffSide, path: string): string {
  const commitId = side === 'old' ? config.beforeCommitId! : config.afterCommitId;
  return `${view === 'src' ? config.repoLink : config.rawLink}/${view}/commit/${encodeURIComponent(commitId)}/${pathEscapeSegments(path)}`;
}

// the size is counted while reading, a compressed or chunked response has no content-length to check
async function fetchLines(url: string, maxSize: number, tooLargeMessage: string): Promise<string[]> {
  const response = await fetchOk(url);
  if (maxSize < 0) return splitLines(await response.text());
  const reader = response.body!.getReader();
  const chunks: Uint8Array<ArrayBuffer>[] = [];
  let size = 0;
  for (let result = await reader.read(); !result.done; result = await reader.read()) {
    size += result.value.length;
    if (size > maxSize) {
      await reader.cancel();
      throw new Error(tooLargeMessage);
    }
    chunks.push(result.value);
  }
  return splitLines(await new Blob(chunks).text());
}

// both versions of a text file, each up to maxSize bytes (-1 for no limit)
export async function fetchFileContent(config: DiffViewerConfig, file: DiffStreamFile, maxSize: number, tooLargeMessage: string): Promise<FileContent> {
  const [oldLines, newLines] = await Promise.all([
    config.beforeCommitId && file.type !== 'added' ? fetchLines(fileUrl(config, 'raw', 'old', file.oldName), maxSize, tooLargeMessage) : null,
    file.type === 'deleted' ? [] : fetchLines(fileUrl(config, 'raw', 'new', file.name), maxSize, tooLargeMessage),
  ]);
  return {old: oldLines, new: newLines};
}

// the diff of the LFS objects behind the pointers, null when one of them is binary
export async function fetchLfsHunks(config: DiffViewerConfig, file: DiffStreamFile): Promise<DiffHunk[] | null> {
  const {lfs, oldName, name} = file;
  const urls = [lfs!.old?.exists ? fileUrl(config, 'media', 'old', oldName) : null, lfs!.new?.exists ? fileUrl(config, 'media', 'new', name) : null];
  const read = async (url: string | null, headers?: Record<string, string>) => (url ? new Uint8Array(await (await fetchOk(url, {headers})).arrayBuffer()) : new Uint8Array());
  const starts = await Promise.all(urls.map((url) => read(url, {Range: `bytes=0-${lfsSniffLength - 1}`})));
  if (starts.some((data) => data.subarray(0, lfsSniffLength).includes(0))) return null;
  // a short start is the whole object
  const [oldData, newData] = await Promise.all(urls.map(async (url, idx) => (starts[idx].length < lfsSniffLength ? starts[idx] : read(url))));
  const decoder = new TextDecoder();
  return buildHunks(splitLines(decoder.decode(oldData)), splitLines(decoder.decode(newData)));
}

// streams one file without the per-file size limit, null when it's beyond the total limit
export async function fetchFullFile(config: DiffViewerConfig, file: DiffStreamFile): Promise<DiffStreamFile | null> {
  const url = new URL(config.streamUrl, window.location.href);
  url.searchParams.append('files', file.name);
  if (file.oldName !== file.name) url.searchParams.append('files', file.oldName);
  url.searchParams.set('full', 'true');
  const files: DiffStreamFile[] = [];
  await streamDiff(url.href, (streamed) => { files.push(streamed) });
  const loaded = files.find((streamed) => streamed.name === file.name);
  return loaded && !loaded.tooLarge ? loaded : null;
}
