import "./syntax.js";

export function initEditor(value = "", wrapperId = "editor-wrapper", readOnly = false) {
	const wrapper = document.getElementById(wrapperId);

	const editor = CodeMirror(wrapper, {
		mode: "uxnsmal",
		lineNumbers: !readOnly,
		indentUnit: 4,
		tabSize: 4,
		indentWithTabs: true,
		smartIndent: true,
		autofocus: !readOnly,
		showTrailingSpace: !readOnly,
		fixedGutter: false,
		lineWrapping: true,
		value,
		readOnly,
		cursorHeight: readOnly ? 0 : 1,
	});

	return editor;
}
