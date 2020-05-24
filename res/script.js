let currentlyShown;
let originalHref;

function showMedia(href, mediaClass) {
    const modal = document.getElementById("slideshow");
    if (mediaClass === "video") {
        modal.innerHTML = `<video controls="true" autoplay="true">
            <source src="${href}" /></video>`;
    } else if (mediaClass === "audio") {
        modal.innerHTML = `<audio controls="true" autoplay="true">
            <source src="${href}" /></audio>`;
    } else {
        modal.innerHTML = `<img src="${href}" />`;
    }
    modal.style.display = "block";
}

function showSlide(elem, mediaClass) {
    if (!currentlyShown) {
        originalHref = document.location.href;
    }
    currentlyShown = elem;
    const href = elem.href;
    showMedia(href, mediaClass);
    elem.focus();
    history.pushState({"url": href, "className": mediaClass}, href, href);
    return false;
}

function historyHandle(ev) {
    if (ev.state) {
        showMedia(ev.state.url, ev.state.className);
        document.getElementById("slideshow").style.display = "block";
    } else {
        document.getElementById("slideshow").style.display = "none";
        document.getElementById("slideshow").innerHTML = "";
    }
}

function findPrev(current) {
    if (!current) {
        return false;
    }
    const prevLi = current.parentNode.previousElementSibling;
    if (!prevLi) {
        return false;
    } else if (prevLi.className === 'folder' || prevLi.className === "") {
        return findPrev(prevLi.querySelector("a"));
    }
    return prevLi.querySelector("a");
}

function prevSlide() {
    if (!currentlyShown) {
        return;
    }
    const prev = findPrev(currentlyShown);
    if (prev) {
        prev.click();
    } else {
        cancelSlide();
    }
}

function findNext(current) {
    if (!current) {
        return false;
    }
    const nextLi = current.parentNode.nextElementSibling;
    if (!nextLi) {
        return false;
    } else if (nextLi.className === 'folder' || nextLi.className === "") {
        return findNext(nextLi.querySelector("a"));
    }
    return nextLi.querySelector("a");
}

function nextSlide() {
    if (!currentlyShown) {
        return;
    }
    const next = findNext(currentlyShown);
    if (next) {
        next.click();
    } else {
        cancelSlide();
    }
}

function cancelSlide() {
    const modal = document.getElementById("slideshow");
    modal.style.display = "none";
    modal.innerHTML = "";
    currentlyShown = null;
    history.pushState(null, originalHref, originalHref);
}

function clickHandle(ev) {
    if (ev && ["IMG", "VIDEO", "AUDIO"].includes(ev.target.tagName)) {
        return false;
    }
    cancelSlide();
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
    window.addEventListener("keydown", keyHandle);
    window.addEventListener("popstate", historyHandle);
    document.getElementById("slideshow").addEventListener("click", clickHandle);
}
window.addEventListener("load", init)
