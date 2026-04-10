export async function fetchSnippetSource(id) {
	const res = await fetch(`/api/snippet?id=${id}`);
	const text = await res.text();
	if (!res.ok) {
		throw new Error(text);
	}

	return text
}

