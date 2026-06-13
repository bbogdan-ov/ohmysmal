let timeout = -1;

function toggleElement(id, toggle = true) {
	const el = document.getElementById(id);
	if (!el) return null;

	if (toggle) {
		el.classList.toggle("hidden");
	} else {
		el.classList.remove("hidden");
	}

	return el;
}

function openAuthForms(toggle = false) {
	const forms = toggleElement("header-auth-forms", toggle);
	if (!forms) return;

	forms.scrollIntoView({
		behavior: "smooth",
		block: "center",
	});
}

function setErrorPopup(status, msg) {
	const popup = document.getElementById("error-popup");
	const popupText = document.getElementById("error-popup-text");
	popup.classList.add("active");

	if (400 <= status && status < 500 || status <= 0) {
		popupText.textContent = `${msg}`;
	} else if (500 <= status && status < 600) {
		popupText.textContent = `Server error: ${msg}`;
	} else {
		popupText.textContent = `Unknown error ${status}: ${msg}`;
	}

	clearTimeout(timeout);
	timeout = setTimeout(() => {
		popup.classList.remove("active");
	}, 6000);
}

document.body.addEventListener("htmx:afterRequest", e => {
	if (e.detail.successful) return;

	const status = e.detail.xhr.status;
	if (300 <= status && status < 400) return;

	setErrorPopup(status, e.detail.xhr.response);
})
