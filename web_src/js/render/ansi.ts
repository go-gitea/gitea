import {AnsiUp} from 'ansi_up';

const replacements: Array<[RegExp, string]> = [
  [/\x1b\[\d+[A-H]/g, ''], // Move cursor, treat them as no-op
  [/\x1b\[\d?[JK]/g, '\r'], // Erase display/line, treat them as a Carriage Return
];

// render ANSI to HTML
export function renderAnsi(line: string): string {
  // create a fresh ansi_up instance because otherwise previous renders can influence
  // the output of future renders, because ansi_up is stateful and remembers things like
  // unclosed opening tags for colors.
  const ansi_up = new AnsiUp();
  ansi_up.use_classes = true;

  if (line.endsWith('\r\n')) {
    line = line.substring(0, line.length - 2);
  } else if (line.endsWith('\n')) {
    line = line.substring(0, line.length - 1);
  }

  if (line.includes('\x1b')) {
    for (const [regex, replacement] of replacements) {
      line = line.replace(regex, replacement);
    }
  }

  if (!line.includes('\r')) {
    return ansi_up.ansi_to_html(line);
  }

  // handle "\rReading...1%\rReading...5%\rReading...100%",
  // convert it into a multiple-line string: "Reading...1%\nReading...5%\nReading...100%"
  const lines = [];
  for (const part of line.split('\r')) {
    if (part === '') continue;
    const partHtml = ansi_up.ansi_to_html(part);
    if (partHtml !== '') {
      lines.push(partHtml);
    }
  }

  // the log message element is with "white-space: break-spaces;", so use "\n" to break lines
  return lines.join('\n');
}
