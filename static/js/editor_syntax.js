// UXNSMAL mode for Code Mirror, based on Lua and C-like mode.

CodeMirror.defineMode("uxnsmal", function(config, parserConfig) {
	var indentUnit = config.indentUnit;

	function prefixRE(words) {
		return new RegExp("^(?:" + words.join("|") + ")", "i");
	}
	function wordRE(words) {
		return new RegExp("^(?:" + words.join("|") + ")$", "i");
	}
	var specials = wordRE(["byte", "short"]);

	var builtins = new RegExp("^(?:(add|sub|mul|div|inc|shift|and|or|xor|eq|neq|gth|lth|pop|swap|nip|rot|dup|over|sth|load|store|call|input|input2|output)(?:-r|-k|-rk|-kr)?)$", "i");
	var keywords = wordRE([
		"fun", "var", "const", "data", "type", "enum", "struct", "alias",
		"break", "loop", "return", "rom", "include", "if", "elif", "else"
	]);

	var indentTokens = wordRE(["\\(", "{"]);
	var dedentTokens = wordRE(["\\)", "}"]);

	function normal(stream, state) {
		var ch = stream.next();

		if (ch == "/") {
			if (stream.eat("/")) {
				stream.skipToEnd();
				return "comment";
			}
			if (stream.eat("*")) {
				return (state.cur = comment)(stream, state);
			}
		}

		if (ch == "\"" || ch == "'")
			return (state.cur = string(ch))(stream, state);

		if (/\d/.test(ch)) {
			stream.eatWhile(/[\w.%\*]/);
			return "number";
		}

		if (/[\w_]/.test(ch)) {
			stream.eatWhile(/[\w\\\-_.]/);
			return "variable";
		}

		return null;
	}

	function comment(stream, state) {
		var end = false, ch;
		while (ch = stream.next()) {
			if (ch == "/" && end) {
				state.cur = normal;
				break;
			}
			end = ch == "*";
		}
		return "comment";
	}

	function string(quote) {
		return function(stream, state) {
			var escaped = false, ch;
			while ((ch = stream.next()) != null) {
				if (ch == quote && !escaped) break;
				escaped = !escaped && ch == "\\";
			}
			if (!escaped) state.cur = normal;
			return "string";
		};
	}

	return {
		startState: function(basecol) {
			return {basecol: basecol || 0, indentDepth: 0, cur: normal};
		},

		token: function(stream, state) {
			if (stream.eatSpace()) return null;
			var style = state.cur(stream, state);
			var word = stream.current();
			if (style == "variable") {
				if (keywords.test(word)) style = "keyword";
				else if (builtins.test(word)) style = "builtin";
				else if (specials.test(word)) style = "variable-2";
			}
			if ((style != "comment") && (style != "string")){
				if (indentTokens.test(word)) ++state.indentDepth;
				else if (dedentTokens.test(word)) --state.indentDepth;
			}
			return style;
		},

		indent: function(state, textAfter) {
			return state.basecol + indentUnit * state.indentDepth;
		},

		lineComment: "//",
		blockCommentStart: "/*",
		blockCommentEnd: "*/",
	};
});

