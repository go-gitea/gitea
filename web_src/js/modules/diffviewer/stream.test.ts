import {GET} from '../fetch.ts';
import {streamDiff, type DiffStreamFile} from './stream.ts';

vi.mock('../fetch.ts', () => ({GET: vi.fn()}));

function respond(text: string) {
  const bytes = new TextEncoder().encode(text);
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (let i = 0; i < bytes.length; i += 5) controller.enqueue(bytes.slice(i, i + 5)); // splits lines and multi-byte characters
      controller.close();
    },
  });
  vi.mocked(GET).mockResolvedValue(new Response(body));
}

test('streamDiff', async () => {
  const files: DiffStreamFile[] = [];
  respond('{"name":"ä.txt","hunksSize":1}\n{"name":"b","hunksSize":2}\n{"end":true,"truncated":true,"failed":false}');
  expect(await streamDiff('/stream', (file) => { files.push(file) })).toEqual({end: true, truncated: true, failed: false});
  expect(files.map((file) => file.name)).toEqual(['ä.txt', 'b']);

  // without its end line, the stream broke off
  respond('{"name":"a","hunksSize":1}\n');
  expect((await streamDiff('/stream', () => {})).failed).toBe(true);
});
