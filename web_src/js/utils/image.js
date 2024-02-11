export async function pngChunks(blob) {
  const uint8arr = new Uint8Array(await blob.arrayBuffer());
  const chunks = [];
  if (uint8arr.length < 8) return chunks;
  const view = new DataView(uint8arr.buffer);
  if (view.getBigUint64(0) !== 9894494448401390090n) return chunks;

  const decoder = new TextDecoder();
  let index = 8;
  while (index < uint8arr.length) {
    const len = view.getUint32(index);
    chunks.push({
      name: decoder.decode(uint8arr.slice(index + 4, index + 8)),
      data: uint8arr.slice(index + 8, index + 8 + len),
    });
    index += len + 12;
  }

  return chunks;
}

export async function pngInfo(blob) {
  let width = 0;
  let dppx = 1;
  if (blob.type !== 'image/png') return {width, dppx};

  try {
    for (const {name, data} of await pngChunks(blob)) {
      const view = new DataView(data.buffer);
      if (name === 'IHDR' && data?.length) {
        // extract width from mandatory IHDR chunk
        width = view.getUint32(0);
      } else if (name === 'pHYs' && data?.length) {
        // extract dppx from optional pHYs chunk, assuming pixels are square
        const unit = view.getUint8(8);
        if (unit === 1) {
          dppx = Math.round(view.getUint32(0) / 39.3701) / 72; // meter to inch to dppx
        }
      }
    }
  } catch {}

  return {width, dppx};
}
