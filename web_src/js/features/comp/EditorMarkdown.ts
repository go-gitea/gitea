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

function handleIndentSelection(textarea, e) {
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

function handleNewline(textarea: HTMLTextAreaElement, e: Event) {
  const selStart = textarea.selectionStart;
  const selEnd = textarea.selectionEnd;
  if (selEnd !== selStart) return; // do not process when there is a selection

  const value = textarea.value;

  // find the current line
  // * if selStart is 0, lastIndexOf(..., -1) is the same as lastIndexOf(..., 0)
  // * if lastIndexOf reruns -1, lineStart is 0 and it is still correct.
  const lineStart = value.lastIndexOf('\n', selStart - 1) + 1;
  let lineEnd = value.indexOf('\n', selStart);
  lineEnd = lineEnd < 0 ? value.length : lineEnd;
  let line = value.slice(lineStart, lineEnd);
  if (!line) return; // if the line is empty, do nothing, let the browser handle it

  // parse the indention
  const indention = /^\s*/.exec(line)[0];
  line = line.slice(indention.length);

  // parse the prefixes: "1. ", "- ", "* ", there could also be " [ ] " or " [x] " for task lists
  // there must be a space after the prefix because none of "1.foo" / "-foo" is a list item
  const prefixMatch = /^([0-9]+\.|[-*])(\s\[([ x])\])?\s/.exec(line);
  let prefix = '';
  if (prefixMatch) {
    prefix = prefixMatch[0];
    if (lineStart + prefix.length > selStart) prefix = ''; // do not add new line if cursor is at prefix
  }

  line = line.slice(prefix.length);
  if (!indention && !prefix) return; // if no indention and no prefix, do nothing, let the browser handle it

  e.preventDefault();
  if (!line) {
    // clear current line if we only have i.e. '1. ' and the user presses enter again to finish creating a list
    textarea.value = value.slice(0, lineStart) + value.slice(lineEnd);
    textarea.setSelectionRange(selStart - prefix.length, selStart - prefix.length);
  } else {
    // start a new line with the same indention and prefix
    let newPrefix = prefix;
    // a simple approach, otherwise it needs to parse the lines after the current line
    if (/^\d+\./.test(prefix)) newPrefix = `1. ${newPrefix.slice(newPrefix.indexOf('.') + 2)}`;
    newPrefix = newPrefix.replace('[x]', '[ ]');
    const newLine = `\n${indention}${newPrefix}`;
    textarea.value = value.slice(0, selStart) + newLine + value.slice(selEnd);
    textarea.setSelectionRange(selStart + newLine.length, selStart + newLine.length);
  }
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
