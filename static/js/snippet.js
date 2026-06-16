export async function fetchSnippetSource(id) {
	const res = await fetch(`/snippets/${id}.smal`);
	const text = await res.text();
	if (!res.ok) {
		throw new Error(text);
	}

	return text
}

