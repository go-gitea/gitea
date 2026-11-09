import {detectEscapes, loadEscapeTables} from './escape.ts';
import {segmentsToHtml} from './render.ts';

const title = (char: string, confusable: string) => `${char} can be confused with ${confusable}`;

test('detectEscapes', async () => {
  const tables = await loadEscapeTables('en-US');
  expect(detectEscapes('plain ascii', tables, title)).toBeNull();

  const invisible = detectEscapes('a\u{200B}b', tables, title)!;
  expect(invisible.hasInvisible).toBe(true);
  expect(segmentsToHtml([['a\u{200B}b', 'tok-string']], [], '', invisible.escapes)).toEqual(
    '<span class="tok-string">a</span><span class="tok-string"><span class="escaped-code-point" data-escaped="[U+200B]"><span class="char">\u{200B}</span></span></span><span class="tok-string">b</span>',
  );

  // a cyrillic "а" in an ASCII word is ambiguous, in a cyrillic sentence it isn't
  const ambiguous = detectEscapes('vаr x', tables, title)!;
  expect(ambiguous.hasAmbiguous).toBe(true);
  expect(ambiguous.escapes[0].html).toContain('data-tooltip-content="а [U+0430] can be confused with a [U+0061]"');
  expect(detectEscapes('съешь же ещё этих мягких французских булок, да выпей чаю', tables, title)).toBeNull();

  expect(detectEscapes('a\x00b', tables, title)!.escapes[0].html).toEqual('<span class="broken-code-point" data-escaped="␀"><span class="char">\x00</span></span>');
});
