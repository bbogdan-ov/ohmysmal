import { initCompiler } from "./compiler.js";
import { editorConfig } from "./editor.js";

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

let emu;
let publishModal;
let previewUrl = null;

async function init() {
	let changed = false;
	let errorsCount = 0;
	let startTime = 0;
	let prevZoom = 0;

	let wantsToPublish = false;

	// Init the UXN VARVARA emulator.
	emu = new Emu();
	emu.init();

	publishModal = document.getElementById("editor_publish_modal");
	const runButton = document.getElementById("run_button");

	// Init the text editor.
	const editorStats = document.getElementById("editor_stats");
	const editor = CodeMirror.fromTextArea(
		document.getElementById("snippet_source"),
		editorConfig(),
	);
	if (editor.getValue() == "") {
		editor.setValue(DEFAULT_CODE)
	}

	function updateStats() {
		const { line, ch } = editor.getCursor();
		editorStats.textContent = `${line+1}:${ch+1}`;
	}
	updateStats();
	editor.on("cursorActivity", updateStats);
	editor.on("change", () => changed = true);

	// Init display window.
	const win = initDisplayWindow(emu, editor);

	const problems = document.getElementById("editor_problems");

	function addMessage(msg, className) {
		const m = document.createElement("p");
		m.textContent = msg;
		m.className = className ?? "info";
		problems.append(m);
	}
	function addProblem(line, col, msg) {
		addMessage(`${line+1}:${col+1}: error: ${msg}`, "error");

		if (errorsCount == 0) {
			onError();
		}

		errorsCount += 1;
		wantsToPublish = false;
	}
	function addNote(line, col, msg) {
		addMessage(`${line+1}:${col+1}: note: ${msg}`, "note");
	}

	function recompile(focus=false) {
		problems.innerHTML = "";

		prevZoom = emu.screen.zoom;
		errorsCount = 0;
		startTime = Date.now();

		addMessage("Compiling...")
		compile(editor.doc.getValue());
		if (focus) win.focus();
	}
	function load(program) {
		const elapsed = Date.now() - startTime;
		addMessage(`Compiled ${program.length} bytes in ${elapsed}ms!`);
		emu.load(program);
		emu.screen.set_zoom(prevZoom);

		onCompiledSuccessfully();
	}

	// Init the UXNSMAL compiler.
	setLoadingText("Loading the UXNSMAL compiler...");
	const { compile } = await initCompiler(load, addProblem, addNote);

	setLoadingText("Compiling the snippet...");
	recompile();

	initPublishForm(editor);
	initUploadButton(editor);

	editor.setOption("extraKeys", {
		"Ctrl-Enter": recompile.bind(true),
	});
	runButton.addEventListener("click", recompile.bind(true));

	setLoadingText("Done!");
	setLoadingText(null);

	// TODO: save and load code in the local storage.
	// Prevent the editor from closing with unsaved changes.
	window.addEventListener("beforeunload", e => {
		if (!changed) return;
		e.preventDefault();
		e.returnValue = "";
	});

	window.addEventListener("keydown", e => {
		if (publishModal.classList.contains("open") && e.ctrlKey && e.code == "KeyP") {
			e.preventDefault();
			reloadPublishPreview();
		}
	})

	function onCompiledSuccessfully() {
		console.log("Compiled successfully");

		if (wantsToPublish)
			publish();
	}
	function onError() {
		if (wantsToPublish)
			setErrorPopup(0, "You have compilation errors! Fix them before publishing!");
	}

	async function publish() {
		const form = document.getElementById("editor_publish_form");
		if (!form) return;

		form.classList.add("htmx-request");

		const data = new FormData(form);
		await formAppendSnippetData(data, editor)

		const res = await fetch("/api/snippet", {
			method: "POST",
			body: data,
		});
		const text = await res.text();

		form.classList.remove("htmx-request");

		if (!res.ok) {
			setErrorPopup(res.status, text);
			return;
		}

		changed = false;
		window.location.replace(`/snippet?id=${text}`);
	}

	function initPublishForm(editor) {
		const form = document.getElementById("editor_publish_form");
		if (!form) return;

		async function onSubmit(e) {
			e.preventDefault();

			wantsToPublish = true;
			recompile()
		}

		form.addEventListener("submit", onSubmit);
	}

	function initUploadButton(editor) {
		const uploadButton = document.getElementById("upload_button");
		if (!uploadButton) return;

		const params = new URLSearchParams(new URL(window.location.href).search);
		const snippetId = params.get("snippet");
		if (!snippetId) {
			console.warn(`No "snippet" url param was provided.`);
			return;
		}

		async function uploadChanges() {
			const data = new FormData();
			await formAppendSnippetData(data, editor)

			const res = await fetch(`/api/snippet?id=${snippetId}`, {
				method: "PATCH",
				body: data,
			});
			const text = await res.text();
			if (!res.ok) {
				setErrorPopup(res.status, text);
				return;
			}

			window.location.replace(`/snippet?id=${snippetId}`);
		}

		uploadButton.addEventListener("click", uploadChanges);
}
}

function initDisplayWindow(emu, editor) {
	const win = document.getElementById("display_window");
	const zoomButton = document.getElementById("display_zoom_button");

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

async function formAppendSnippetData(formData, editor) {
	if (!previewUrl) {
		setErrorPopup("No image preview, try again");
		return;
	}

	const value = editor.getValue().trim();
	const sourceBlob = new Blob([value], {
		type: "text/plain; charset=utf-8"
	});

	const res = await fetch(previewUrl);
	const previewBlob = await res.blob();

	formData.append("file", sourceBlob, "source.smal");
	formData.append("preview", previewBlob, "preview.png");
}

function setLoadingText(text) {
	const loader = document.getElementById("loader");
	const loaderText = document.getElementById("loader_text");

	if (!text) {
		loader.classList.add("hidden");
		return;
	}

	loaderText.textContent = text;
	loader.classList.remove("hidden");
}

function togglePublishModal() {
	if (!publishModal.classList.contains("open")) {
		reloadPublishPreview();
	}

	publishModal.classList.toggle("open");
}

function reloadPublishPreview() {
	const previewImg = document.getElementById("editor_publish_preview");
	previewUrl = emu.screen.display.toDataURL();
	previewImg.src = previewUrl;
}

window.togglePublishModal = togglePublishModal;
window.reloadPublishPreview = reloadPublishPreview;

document.addEventListener("DOMContentLoaded", init);
