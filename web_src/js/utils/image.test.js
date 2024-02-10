import {pngChunks} from './image.js';

test('pngChunks', async () => {
  const blob = await (await globalThis.fetch('data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAADUlEQVQIHQECAP3/AAAAAgABzePRKwAAAABJRU5ErkJggg==')).blob();
  expect(pngChunks(new Uint8Array(await blob.arrayBuffer()))).toEqual([
    {name: "IHDR", data: new Uint8Array([0, 0, 0, 1, 0, 0, 0, 1, 8, 0, 0, 0, 0])},
    {name: "IDAT", data: new Uint8Array([8, 29, 1, 2, 0, 253, 255, 0, 0, 0, 2, 0, 1])},
    {name: "IEND", data: new Uint8Array([])},
  ]);
});
