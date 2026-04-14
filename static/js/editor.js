import "./syntax.js";

export function editorConfig(value = "", readOnly = false) {
	return {
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
	}
}
