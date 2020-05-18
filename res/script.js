let currentlyShown;

function showSlide(elem) {
    currentlyShown = elem;
    const modal = document.getElementById("slideshow");
    modal.innerHTML = `<img src="${elem.href}" />`
    modal.style.display = "block";
    return false;
}

function prevSlide() {
    if (!currentlyShown) {
        return;
    }
    console.log(currentlyShown);
}

function nextSlide() {
    if (!currentlyShown) {
        return;
    }
    console.log(currentlyShown);
}

function cancelSlide() {
    const modal = document.getElementById("slideshow");
    modal.style.display = "none";
    modal.innerHTML = "";
    currentlyShown = null;
    return false;
}

function keyHandle(ev) {
    if (!currentlyShown) {
        return;
    }
    switch (ev.code) {
    case "Backspace":
    case "Delete":
    case "KeyQ":
    case "Escape":
        return cancelSlide();
    case "PageUp":
    case "ArrowUp":
    case "ArrowLeft":
        return prevSlide();
    case "PageDown":
    case "ArrowDown":
    case "ArrowRight":
        return nextSlide();
    case "Tab":
    case "Space":
    case "Enter":
        return ev.shiftKey ? prevSlide() : nextSlide();
    // case "Home":
    // case "End":
    }
}

function init() {
    console.log ("initializing")
    window.addEventListener("keydown", keyHandle);
    document.getElementById("slideshow").addEventListener("click", cancelSlide)
}
window.addEventListener("load", init)
