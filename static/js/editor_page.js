import { initCompiler } from "./compiler.js";
import { fetchSnippetSource } from "./snippet.js";
import { initEditor } from "./editor.js";

const DEFAULT_CODE = `\
// hello

// Note that UXNSMAL is VERY INCOMPLETE!
// Anything could be changed without any notice.
// These is also no documentation, you're on your own.
// Good luck.
//
// http://github.com/bbogdan-ov/uxnsmal
//
// Hot keys:
//     Ctrl-Enter - Compile and run

fun on-reset ( -> ) {
	// The entry point...
}

// Here is all the devices you need to start doing things:
alias enum byte System {
	expansion { 0x02 }
	wst       { 0x04 }
	rst       { 0x05 }
	metadata  { 0x06 }
	red       { 0x08 }
	green     { 0x0a }
	blue      { 0x0c }
	debug     { 0x0e }
	state     { 0x0f }
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

alias enum byte Controller {
	vector { 0x80 }
	button { 0x82 }
	key    { 0x83 }
}

alias enum byte Mouse {
	vector  { 0x90 }
	x       { 0x92 }
	y       { 0x94 }
	state   { 0x96 }
	scrollx { 0x9a }
	scrolly { 0x9c }
}

alias enum byte Datetime {
	year   { 0xc0 }
	month  { 0xc2 }
	day    { 0xc3 }
	hour   { 0xc4 }
	minute { 0xc5 }
	second { 0xc6 }
	dotw   { 0xc7 }
	doty   { 0xc8 }
	isdst  { 0xca }
}`;

async function init() {
	// Init the UXN VARVARA emulator.
	const emu = new Emu();
	emu.init();

	let publishModalOpen = false;
	const publishModal = document.getElementById("editor-publish-modal");
	const publishButton = document.getElementById("publish-button");
	const runButton = document.getElementById("run-button");

	// Init the text editor.
	const editorStats = document.getElementById("editor-stats");
	const editor = initEditor(DEFAULT_CODE);
	let changed = false;

	function updateStats() {
		const { line, ch } = editor.getCursor();
		editorStats.textContent = `${line+1}:${ch+1}`;
	}
	updateStats();
	editor.on("cursorActivity", updateStats);
	editor.on("change", () => changed = true);

	// Init display window.
	const win = initDisplayWindow(emu, editor);

	const problems = document.getElementById("editor-problems");

	function addMessage(msg, className) {
		const m = document.createElement("p");
		m.textContent = msg;
		m.className = className ?? "info";
		problems.append(m);
	}
	function addProblem(line, col, msg) {
		addMessage(`${line+1}:${col+1}: error: ${msg}`, "error");
	}
	function addNote(line, col, msg) {
		addMessage(`${line+1}:${col+1}: note: ${msg}`, "note");
	}

	let startTime = 0;

	function recompile(focus=false) {
		problems.innerHTML = "";

		startTime = Date.now();
		addMessage("Compiling...")

		compile(editor.doc.getValue());
		if (focus) win.focus();
	}
	function load(program) {
		const elapsed = Date.now() - startTime;
		addMessage(`Compiled ${program.length} bytes in ${elapsed}ms!`);
		emu.load(program);
	}

	// Init the UXNSMAL compiler.
	setLoadingText("Loading the UXNSMAL compiler...");
	const { compile } = await initCompiler(load, addProblem, addNote);

	// Load snippet source code if any.
	const params = new URLSearchParams(new URL(window.location.href).search);
	const snippetId = params.get("snippet");
	if (snippetId != null) {
		setLoadingText("Loading snippet source code...");

		try {
			const source = await fetchSnippetSource(snippetId);
			editor.setValue(source);
		} catch (err) {
			editor.setValue(`// ${err}\n// Failed to load the snippet...`);
		}

		changed = false;
	}

	setLoadingText("Compiling the snippet...");
	recompile();

	initPublishForm(editor);

	editor.setOption("extraKeys", {
		"Ctrl-Enter": recompile.bind(true),
	});
	runButton.addEventListener("click", recompile.bind(true));

	setLoadingText("Done!");
	setLoadingText(null);

	// TODO: save and load code in the local storage.
	// Prevent the editor from closing with unsaved changes.
	window.addEventListener('beforeunload', function (e) {
		if (!changed) return;
		e.preventDefault();
		e.returnValue = '';
	});

	if (publishButton && publishModal) {
		publishButton.addEventListener("click", () => {
			publishModal.classList.add("open");
			publishModalOpen = true;
		})
		window.addEventListener("pointerdown", e => {
			if (!publishModalOpen) return;

			if (!e.target.closest("#editor-publish-modal")) {
				publishModal.classList.remove("open");
				publishModalOpen = false;
			}
		})
	}
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

		emu.screen.switch_zoom();
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

function initPublishForm(editor) {
	const form = document.getElementById("editor-publish-form");
	if (!form) return;

	async function onSubmit(e) {
		e.preventDefault();

		const blob = new Blob([editor.getValue()], {
			type: "text/plain; charset=utf-8"
		});

		const data = new FormData(form);
		data.append("file", blob, "source.smal");

		const res = await fetch("/api/snippet", {
			method: "POST",
			body: data,
		});
		const text = await res.text();
		if (!res.ok) {
			// TODO: show a proper message to the user.
			console.error("Failed to post the snippet.");
			console.error(text);
			return;
		}
		window.location.replace(`/snippet?id=${text}`);
	}

	form.addEventListener("submit", onSubmit);
}

function setLoadingText(text) {
	const loader = document.getElementById("loader");
	const loaderText = document.getElementById("loader-text");

	if (!text) {
		loader.classList.add("hidden");
		return;
	}

	loaderText.textContent = text;
	loader.classList.remove("hidden");
}

document.addEventListener("DOMContentLoaded", init);
