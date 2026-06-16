import { initCompiler } from "./compiler.js";
import { fetchSnippetSource } from "./snippet.js";
import { editorConfig } from "./editor.js";

async function init() {
	const container = document.getElementById("container");
	const zoomButton = document.getElementById("display-zoom-button");

	const emu = new Emu();
	emu.init();

	function updateZoomButton() {
		zoomButton.textContent = `x${emu.screen.zoom}`;
	}

	zoomButton.onclick = () => {
		const width = emu.screen.display.width;
		const height = emu.screen.display.height;
		const zoom = emu.screen.zoom + 1;
		if (width * zoom > innerWidth || height * zoom > innerHeight - 100) {
			emu.screen.set_zoom(1);
		} else {
			emu.screen.set_zoom(zoom);
		}
		updateZoomButton();
	}

	function load(program) {
		emu.load(program);
	}

	// Init the text editor.
	const source = document.getElementById("snippet_source").value;
	const editor = CodeMirror(
		document.getElementById("editor-wrapper"),
		editorConfig("", true),
	);

	const FIRST_N_LINES = 16;
	const linesLeft = Math.max(countLines(source) - FIRST_N_LINES, 0);
	if (linesLeft > 0) {
		const s = sliceLines(source, FIRST_N_LINES) + `\n\n// ${linesLeft} more lines...`
		editor.setValue(s);
	} else {
		editor.setValue(source);
	}

	const { compile } = await initCompiler(load);
	compile(source);

	const contWidth = container.getBoundingClientRect().width;
	const targetSize = Math.min(contWidth, innerWidth, innerHeight - 100);

	const zoom = Math.floor(targetSize / display.width);
	emu.screen.set_zoom(Math.max(zoom, 1));
	updateZoomButton();
}

function countLines(string) {
	let n = 0;
	for (let i = 0; i < string.length; i ++) {
		if (string[i] == "\n")
			n += 1;
	}
	return n;
}

function sliceLines(string, n) {
	let sliceEnd = 0;
	for (; sliceEnd < string.length; sliceEnd ++) {
		if (string[sliceEnd] == "\n") {
			n -= 1;
			if (n == 0) break;
		}
	}
	return string.slice(0, sliceEnd)
}

document.addEventListener("DOMContentLoaded", init);
