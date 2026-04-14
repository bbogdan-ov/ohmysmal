let timeout = -1;

function openAuthForms(toggle = false) {
	const forms = document.getElementById("header-auth-forms");
	if (!forms) return;

	if (toggle) {
		forms.classList.toggle("hidden");
	} else {
		forms.classList.remove("hidden");
	}

	forms.scrollIntoView({
		behavior: "smooth",
		block: "center",
	});
}

function setErrorPopup(status, msg) {
	const popup = document.getElementById("error-popup");
	const popupText = document.getElementById("error-popup-text");
	popup.classList.add("active");

	if (400 <= status && status < 500) {
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
