const wasmUrl = new URL('plugin.wasm', import.meta.url);
const wasmExecUrl = new URL('wasm_exec.js', import.meta.url);
let wasmBridgePromise;
let styleInjected = false;

function injectScriptOnce(url) {
	return new Promise((resolve, reject) => {
		const existing = document.querySelector(`script[data-go-runtime="${url.href}"]`);
		if (existing) {
			if (existing.dataset.loaded === 'true') {
				resolve();
			} else {
				existing.addEventListener('load', resolve, {once: true});
				existing.addEventListener('error', reject, {once: true});
			}
			return;
		}
		const script = document.createElement('script');
		script.dataset.goRuntime = url.href;
		script.src = url.href;
		script.async = true;
		script.addEventListener('load', () => {
			script.dataset.loaded = 'true';
			resolve();
		}, {once: true});
		script.addEventListener('error', reject, {once: true});
		document.head.appendChild(script);
	});
}

function sleep(ms) {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

async function waitForExport(name, timeoutMs = 2000) {
	const start = Date.now();
	while (typeof globalThis[name] !== 'function') {
		if (Date.now() - start > timeoutMs) {
			throw new Error(`Go runtime did not expose ${name} within ${timeoutMs}ms`);
		}
		await sleep(20);
	}
	return globalThis[name];
}

async function ensureWasmBridge() {
	if (!wasmBridgePromise) {
		wasmBridgePromise = (async () => {
			if (typeof globalThis.Go === 'undefined') {
				await injectScriptOnce(wasmExecUrl);
			}
			if (typeof globalThis.Go === 'undefined') {
				throw new Error('Go runtime (wasm_exec.js) is unavailable');
			}
			const go = new globalThis.Go();
			let result;
			const fetchRequest = fetch(wasmUrl);
			if (WebAssembly.instantiateStreaming) {
				try {
					result = await WebAssembly.instantiateStreaming(fetchRequest, go.importObject);
				} catch (err) {
					console.warn('instantiateStreaming failed; falling back to ArrayBuffer', err);
					const buffer = await (await fetchRequest).arrayBuffer();
					result = await WebAssembly.instantiate(buffer, go.importObject);
				}
			} else {
				const buffer = await (await fetchRequest).arrayBuffer();
				result = await WebAssembly.instantiate(buffer, go.importObject);
			}
			go.run(result.instance);
			const processFile = await waitForExport('wasmProcessFile');
			return {
				process(content) {
					const output = processFile(content);
					return typeof output === 'string' ? output : String(output ?? '');
				},
			};
		})();
	}
	return wasmBridgePromise;
}

async function fetchFileText(fileUrl) {
	const response = await window.fetch(fileUrl, {headers: {'Accept': 'text/plain'}});
	if (!response.ok) {
		throw new Error(`failed to fetch file (${response.status})`);
	}
	return response.text();
}

function ensureStyles() {
	if (styleInjected) return;
	styleInjected = true;
	const style = document.createElement('style');
	style.textContent = `
.go-wasm-renderer {
	font-family: var(--fonts-proportional, system-ui);
	border: 1px solid var(--color-secondary);
	border-radius: 6px;
	overflow: hidden;
}
.go-wasm-renderer__header {
	margin: 0;
	padding: 0.75rem 1rem;
	background: var(--color-secondary-alpha-20);
	font-weight: 600;
}
.go-wasm-renderer pre {
	margin: 0;
	padding: 1rem;
	background: var(--color-box-body);
	font-family: var(--fonts-monospace, SFMono-Regular, monospace);
	white-space: pre;
	overflow-x: auto;
}
.go-wasm-renderer__error {
	color: var(--color-danger);
}
`;
	document.head.appendChild(style);
}

function renderError(container, message) {
	container.innerHTML = '';
	const wrapper = document.createElement('div');
	wrapper.className = 'go-wasm-renderer';
	const header = document.createElement('div');
	header.className = 'go-wasm-renderer__header';
	header.textContent = 'Go WASM Renderer';
	const body = document.createElement('pre');
	body.className = 'go-wasm-renderer__error';
	body.textContent = message;
	wrapper.append(header, body);
	container.appendChild(wrapper);
}

export default {
	name: 'Go WASM Renderer',
	async render(container, fileUrl) {
		ensureStyles();
		try {
			const [bridge, content] = await Promise.all([
				ensureWasmBridge(),
				fetchFileText(fileUrl),
			]);

			const processed = await bridge.process(content);
			const wrapper = document.createElement('div');
			wrapper.className = 'go-wasm-renderer';
			const header = document.createElement('div');
			header.className = 'go-wasm-renderer__header';
			header.textContent = 'Go WASM Renderer';
			const body = document.createElement('pre');
			body.textContent = processed;
			wrapper.append(header, body);
			container.innerHTML = '';
			container.appendChild(wrapper);
		} catch (err) {
			console.error('Go WASM plugin failed', err);
			renderError(container, `Unable to render file: ${err.message}`);
		}
	},
};
