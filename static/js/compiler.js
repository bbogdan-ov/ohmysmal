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

	const { ptr, len } = encodeString(`
// =====================
// SIMPLE SPRITE example.
// =====================

/// VARVARA system device.
alias enum byte System {
	r { 0x08 }
	g { 0x0a }
	b { 0x0c }
}

/// VARVARA screen device.
alias enum byte Screen {
	vector { 0x20 }
	width  { 0x22 }
	height { 0x24 }
	auto   { 0x26 }
	x      { 0x28 }
	y      { 0x2a }
	addr   { 0x2c }
	pixel  { 0x2e }
	sprite { 0x2f }
}

fun on-reset ( -> ) {
	// Set color palette.
	// 0xABCD - red
	// 0xABCD - green
	// 0xABCD - blue
	// 0xAAA - A
	// 0xBBB - B
	// 0xCCC - C
	// 0xDDD - D
	//
	// In this example palette looks like this:
	// 0xFFF - A
	// 0x000 - B
	// 0x7db - C
	// 0xf62 - D
	0xf07f* System.r output
	0xf0d6* System.g output
	0xf0b2* System.b output

	// Set window size.
	// '*' means that number is a short.
	64* Screen.width output
	64* Screen.height output

	// Set current sprite address.
	&my-sprite Screen.addr output
	// Place the sprite somewhere.
	16* Screen.x output
	32* Screen.y output
	// Draw it with fourh (D) color.
	0b00000011 Screen.sprite output
}

// 8x8 1bit sprite.
data my-sprite {
	0b01111110
	0b11111111
	0b10111101
	0b10111101
	0b11111111
	0b10111101
	0b11000011
	0b01111110
}
	`);
	compile_source(ptr, len);
}
