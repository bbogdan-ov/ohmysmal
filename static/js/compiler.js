const U32_BYTES = 4;

let memory;

export async function initCompiler(load, problem, note) {
	const obj = await WebAssembly.instantiateStreaming(
		fetch("/static/wasm/compiler.wasm"),
		{
			env: {
				console_log: (ptr, len) => {
					console.log("RUST SAYS:", decodeString(ptr, len));
				},
				load: (ptr, len) => {
					const bytes = new Uint8Array(memory.buffer, ptr, len);
					load(bytes);
				},
				problem: (line, col, ptr, len) => {
					if (problem) {
						const msg = decodeString(ptr, len);
						problem(line, col, msg)
					}
				},
				note: (line, col, ptr, len) => {
					if (note) {
						const msg = decodeString(ptr, len);
						note(line, col, msg)
					}
				}
			}
		}
	);

	const { alloc, compile_source } = obj.instance.exports;
	memory = obj.instance.exports.memory;

	function decodeString(ptr, len) {
		const bytes = new Uint8Array(memory.buffer, ptr, len);
		return new TextDecoder().decode(bytes);
	}

	function encodeString(jsString) {
		const encoder = new TextEncoder();
		const bytes = encoder.encode(jsString);

		const ptr = U32_BYTES;
		const view = new Uint8Array(memory.buffer, ptr, bytes.length);
		view.set(bytes);

		return { ptr, len: bytes.length };
	}

	return {
		compile: (source) => {
			const { ptr, len } = encodeString(source);
			compile_source(ptr, len);
		}
	}
}
