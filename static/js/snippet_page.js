import { initCompiler } from "./compiler.js";

function addProblem() {}
function addNote() {}

async function init() {
	const emu = new Emu();
	emu.init();

	function load(program) {
		emu.load(program);
	}

	console.log("TODO: loading the uxnsmal compiler");
	const { compile } = await initCompiler(load, addProblem, addNote);

	console.log("TODO: done loading");

	const params = new URLSearchParams(new URL(window.location.href).search);
	const snippetId = params.get("id");
	if (!snippetId) return;

	try {
		const source = await fetchSnippet(snippetId)
		console.log("TODO: source code loaded");
		compile(source);
	} catch (err) {
		console.error("TODO: output error");
		console.error(err);
	}
}

async function fetchSnippet(id) {
	console.log("TODO: Loading the snippet source code...");

	const res = await fetch(`/api/snippet?id=${id}`);
	const text = await res.text();
	if (!res.ok) {
		throw new Error(text);
	}

	return text
}

document.addEventListener("DOMContentLoaded", init);
