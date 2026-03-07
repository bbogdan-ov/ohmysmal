const U32_BYTES = 4;

let memory;

export async function initCompiler(emu) {
	const obj = await WebAssembly.instantiateStreaming(
		fetch("/static/wasm/compiler.wasm"),
		{ env: {
			console_log: (ptr, len) => {
				const bytes = new Uint8Array(memory.buffer, ptr, len);
				const str = new TextDecoder().decode(bytes);
				console.log("Message from Rust:", str);
			},
			load: (ptr, len) => {
				const bytes = new Uint8Array(memory.buffer, ptr, len);
				console.log(`Loaded ${bytes.length} bytes`);
				emu.load(bytes)
			}
		} }
	);

	const { alloc, compile_source } = obj.instance.exports;
	memory = obj.instance.exports.memory;

	function encodeString(jsString) {
		const encoder = new TextEncoder();
		const bytes = encoder.encode(jsString);
		console.log(jsString, bytes.length)

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
