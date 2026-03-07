import { initCompiler } from "./compiler.js";

const DEFAULT_CODE = `\
alias enum byte System {
	red       { 0x08 }
	green     { 0x0a }
	blue      { 0x0c }
}

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

alias enum byte Mouse {
	vector  { 0x90 }
	x       { 0x92 }
	y       { 0x94 }
	state   { 0x96 }
	scrollx { 0x9a }
	scrolly { 0x9c }
}

fun on-reset ( -> ) {
	0xf000* System.red output
	0xf000* System.green output
	0xf000* System.blue output

	256* Screen.width output
	256* Screen.height output

	&on-mouse Mouse.vector output
}

fun on-mouse ( -> ) {
	Mouse.state input 0 eq if { return }

	Mouse.x input2 4* sub Screen.x output
	Mouse.y input2 4* sub Screen.y output
	&circle Screen.addr output
	0b00000101 Screen.sprite output
}

data circle {
	0b00111100
	0b01111110
	0b11111111
	0b11111111
	0b11111111
	0b11111111
	0b01111110
	0b00111100
}`;

async function init() {
	// Init the UXN VARVARA emulator.
	const emu = new Emu();
	emu.init();

	// Init the text editor.
	const editor = initEditor();

	// Init display window.
	const win = initDisplayWindow(emu, editor);

	// Init the UXNSMAL compiler.
	const { compile } = await initCompiler(emu);

	function recompile(focus=false) {
		compile(editor.doc.getValue());
		if (focus) win.focus();
	}

	recompile();

	editor.setOption("extraKeys", {
		"Ctrl-Enter": () => recompile(true)
	});
}

function initEditor() {
	const wrapper = document.getElementById("editor-wrapper");
	const stats = document.getElementById("editor-stats");

	const editor = CodeMirror(wrapper, {
		lineNumbers: true,
		indentUnit: 4,
		tabSize: 4,
		indentWithTabs: true,
		smartIndent: true,
		autofocus: true,
		value: DEFAULT_CODE,
		showTrailingSpace: true,
	});

	function updateStats() {
		const { line, ch } = editor.getCursor();
		stats.textContent = `line ${line+1} : char ${ch+1}`;
	}

	editor.on("cursorActivity", updateStats);

	updateStats();

	return editor;
}

function initDisplayWindow(emu, editor) {
	const win = document.getElementById("display-window");
	const zoomButton = document.getElementById("display-zoom-button");

	const PADDING = 40;
	let pos = { x: PADDING, y: PADDING };

	let pointerPressPos = { x: 0, y: 0};
	let pressPos = { x: 0, y: 0 };

	let isDragging = false;

	updatePos();

	// Window dragging.
	win.addEventListener("pointerdown", e => {
		if (e.target !== win) return;

		pointerPressPos.x = e.clientX;
		pointerPressPos.y = e.clientY;
		pressPos.x = pos.x;
		pressPos.y = pos.y;

		isDragging = true;
	});
	window.addEventListener("pointermove", e => {
		if (!isDragging) return;

		pos.x = pressPos.x - (e.clientX - pointerPressPos.x);
		pos.y = pressPos.y - (e.clientY - pointerPressPos.y);
		updatePos();
	});
	window.addEventListener("pointerup", () => {
		isDragging = false;
	});
	window.addEventListener("resize", () => {
		updatePos();
	})

	// Keymaps.
	win.addEventListener("keydown", e => {
		if (e.key == "Escape") editor.focus();
	});

	// Display canvas scaling.
	zoomButton.addEventListener("click", () => {
		const before = win.getBoundingClientRect();

		emu.screen.toggle_zoom();
		updateStats();

		// Make so that the top-right corner of the window stays in place.
		const after = win.getBoundingClientRect();
		pos.y -= after.height - before.height;
		updatePos();
	});

	function updatePos() {
		const bounds = win.getBoundingClientRect();

		if (pos.x + bounds.width - PADDING <= 0)
			pos.x = -bounds.width + PADDING;
		else if (pos.x + PADDING > innerWidth)
			pos.x = innerWidth - PADDING;

		if (pos.y + bounds.height - PADDING <= 0)
			pos.y = -bounds.height + PADDING;
		else if (pos.y + PADDING > innerHeight)
			pos.y = innerHeight - PADDING;

		win.style.right = pos.x + "px";
		win.style.bottom = pos.y + "px";
	}
	function updateStats() {
		zoomButton.textContent = `x${emu.screen.zoom}`;
	}

	return win;
}

document.addEventListener("DOMContentLoaded", init);
