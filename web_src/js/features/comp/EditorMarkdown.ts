export const EventEditorContentChanged = 'ce-editor-content-changed';

export function triggerEditorContentChanged(target) {
  target.dispatchEvent(new CustomEvent(EventEditorContentChanged, {bubbles: true}));
}

export function textareaInsertText(textarea, value) {
  const startPos = textarea.selectionStart;
  const endPos = textarea.selectionEnd;
  textarea.value = textarea.value.substring(0, startPos) + value + textarea.value.substring(endPos);
  textarea.selectionStart = startPos;
  textarea.selectionEnd = startPos + value.length;
  textarea.focus();
  triggerEditorContentChanged(textarea);
}

type TextareaValueSelection = {
  value: string;
  selStart: number;
  selEnd: number;
}

function handleIndentSelection(textarea: HTMLTextAreaElement, e) {
  const selStart = textarea.selectionStart;
  const selEnd = textarea.selectionEnd;
  if (selEnd === selStart) return; // do not process when no selection

  e.preventDefault();
  const lines = textarea.value.split('\n');
  const selectedLines = [];

  let pos = 0;
  for (let i = 0; i < lines.length; i++) {
    if (pos > selEnd) break;
    if (pos >= selStart) selectedLines.push(i);
    pos += lines[i].length + 1;
  }

  for (const i of selectedLines) {
    if (e.shiftKey) {
      lines[i] = lines[i].replace(/^(\t| {1,2})/, '');
    } else {
      lines[i] = `  ${lines[i]}`;
    }
  }

  // re-calculating the selection range
  let newSelStart, newSelEnd;
  pos = 0;
  for (let i = 0; i < lines.length; i++) {
    if (i === selectedLines[0]) {
      newSelStart = pos;
    }
    if (i === selectedLines[selectedLines.length - 1]) {
      newSelEnd = pos + lines[i].length;
      break;
    }
    pos += lines[i].length + 1;
  }
  textarea.value = lines.join('\n');
  textarea.setSelectionRange(newSelStart, newSelEnd);
  triggerEditorContentChanged(textarea);
}

type MarkdownHandleIndentionResult = {
  handled: boolean;
  valueSelection?: TextareaValueSelection;
}

type TextLinesBuffer = {
  lines: string[];
  lengthBeforePosLine: number;
  posLineIndex: number;
  inlinePos: number
}

export function textareaSplitLines(value: string, pos: number): TextLinesBuffer {
  const lines = value.split('\n');
  let lengthBeforePosLine = 0, inlinePos = 0, posLineIndex = 0;
  for (; posLineIndex < lines.length; posLineIndex++) {
    const lineLength = lines[posLineIndex].length + 1;
    if (lengthBeforePosLine + lineLength > pos) {
      inlinePos = pos - lengthBeforePosLine;
      break;
    }
    lengthBeforePosLine += lineLength;
  }
  return {lines, lengthBeforePosLine, posLineIndex, inlinePos};
}

function markdownReformatListNumbers(linesBuf: TextLinesBuffer, indention: string) {
  const reDeeperIndention = new RegExp(`^${indention}\\s+`);
  const reSameLevel = new RegExp(`^${indention}([0-9]+)\\.`);
  let firstLineIdx: number;
  for (firstLineIdx = linesBuf.posLineIndex - 1; firstLineIdx >= 0; firstLineIdx--) {
    const line = linesBuf.lines[firstLineIdx];
    if (!reDeeperIndention.test(line) && !reSameLevel.test(line)) break;
  }
  firstLineIdx++;
  let num = 1;
  for (let i = firstLineIdx; i < linesBuf.lines.length; i++) {
    const oldLine = linesBuf.lines[i];
    const sameLevel = reSameLevel.test(oldLine);
    if (!sameLevel && !reDeeperIndention.test(oldLine)) break;
    if (sameLevel) {
      const newLine = `${indention}${num}.${oldLine.replace(reSameLevel, '')}`;
      linesBuf.lines[i] = newLine;
      num++;
      if (linesBuf.posLineIndex === i) {
        // need to correct the cursor inline position if the line length changes
        linesBuf.inlinePos += newLine.length - oldLine.length;
        linesBuf.inlinePos = Math.max(0, linesBuf.inlinePos);
        linesBuf.inlinePos = Math.min(newLine.length, linesBuf.inlinePos);
      }
    }
  }
  recalculateLengthBeforeLine(linesBuf);
}

function recalculateLengthBeforeLine(linesBuf: TextLinesBuffer) {
  linesBuf.lengthBeforePosLine = 0;
  for (let i = 0; i < linesBuf.posLineIndex; i++) {
    linesBuf.lengthBeforePosLine += linesBuf.lines[i].length + 1;
  }
}

export function markdownHandleIndention(tvs: TextareaValueSelection): MarkdownHandleIndentionResult {
  const unhandled: MarkdownHandleIndentionResult = {handled: false};
  if (tvs.selEnd !== tvs.selStart) return unhandled; // do not process when there is a selection

  const linesBuf = textareaSplitLines(tvs.value, tvs.selStart);
  const line = linesBuf.lines[linesBuf.posLineIndex] ?? '';
  if (!line) return unhandled; // if the line is empty, do nothing, let the browser handle it

  // parse the indention
  let lineContent = line;
  const indention = /^\s*/.exec(lineContent)[0];
  lineContent = lineContent.slice(indention.length);
  if (linesBuf.inlinePos <= indention.length) return unhandled; // if cursor is at the indention, do nothing, let the browser handle it

  // parse the prefixes: "1. ", "- ", "* ", there could also be " [ ] " or " [x] " for task lists
  // there must be a space after the prefix because none of "1.foo" / "-foo" is a list item
  const prefixMatch = /^([0-9]+\.|[-*])(\s\[([ x])\])?\s/.exec(lineContent);
  let prefix = '';
  if (prefixMatch) {
    prefix = prefixMatch[0];
    if (prefix.length > linesBuf.inlinePos) prefix = ''; // do not add new line if cursor is at prefix
  }

  lineContent = lineContent.slice(prefix.length);
  if (!indention && !prefix) return unhandled; // if no indention and no prefix, do nothing, let the browser handle it

  if (!lineContent) {
    // clear current line if we only have i.e. '1. ' and the user presses enter again to finish creating a list
    linesBuf.lines[linesBuf.posLineIndex] = '';
    linesBuf.inlinePos = 0;
  } else {
    // start a new line with the same indention
    let newPrefix = prefix;
    if (/^\d+\./.test(prefix)) newPrefix = `1. ${newPrefix.slice(newPrefix.indexOf('.') + 2)}`;
    newPrefix = newPrefix.replace('[x]', '[ ]');

    const inlinePos = linesBuf.inlinePos;
    linesBuf.lines[linesBuf.posLineIndex] = line.substring(0, inlinePos);
    const newLineLeft = `${indention}${newPrefix}`;
    const newLine = `${newLineLeft}${line.substring(inlinePos)}`;
    linesBuf.lines.splice(linesBuf.posLineIndex + 1, 0, newLine);
    linesBuf.posLineIndex++;
    linesBuf.inlinePos = newLineLeft.length;
    recalculateLengthBeforeLine(linesBuf);
  }

  markdownReformatListNumbers(linesBuf, indention);
  const newPos = linesBuf.lengthBeforePosLine + linesBuf.inlinePos;
  return {handled: true, valueSelection: {value: linesBuf.lines.join('\n'), selStart: newPos, selEnd: newPos}};
}

function handleNewline(textarea: HTMLTextAreaElement, e: Event) {
  const ret = markdownHandleIndention({value: textarea.value, selStart: textarea.selectionStart, selEnd: textarea.selectionEnd});
  if (!ret.handled) return;
  e.preventDefault();
  textarea.value = ret.valueSelection.value;
  textarea.setSelectionRange(ret.valueSelection.selStart, ret.valueSelection.selEnd);
  triggerEditorContentChanged(textarea);
}

export function initTextareaMarkdown(textarea) {
  textarea.addEventListener('keydown', (e) => {
    if (e.key === 'Tab' && !e.ctrlKey && !e.metaKey && !e.altKey) {
      // use Tab/Shift-Tab to indent/unindent the selected lines
      handleIndentSelection(textarea, e);
    } else if (e.key === 'Enter' && !e.shiftKey && !e.ctrlKey && !e.metaKey && !e.altKey) {
      // use Enter to insert a new line with the same indention and prefix
      handleNewline(textarea, e);
    }
  });
}
