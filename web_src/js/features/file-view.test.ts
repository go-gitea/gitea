import {Buffer} from 'node:buffer';
import {describe, expect, it, vi} from 'vitest';
import {decodeHeadChunk} from './file-view.ts';

describe('decodeHeadChunk', () => {
	it('returns null when input is empty', () => {
		expect(decodeHeadChunk(null)).toBeNull();
		expect(decodeHeadChunk('')).toBeNull();
	});

	it('decodes base64 content into a Uint8Array', () => {
		const data = 'Gitea Render Plugin';
		const encoded = Buffer.from(data, 'utf-8').toString('base64');
		const decoded = decodeHeadChunk(encoded);
		expect(decoded).not.toBeNull();
		expect(new TextDecoder().decode(decoded!)).toBe(data);
	});

	it('logs and returns null for invalid input', () => {
		const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
		const result = decodeHeadChunk('%invalid-base64%');
		expect(result).toBeNull();
		expect(spy).toHaveBeenCalled();
		spy.mockRestore();
	});
});
