"use strict";
(function (global) { /* We're green: no global pollution */
    let currentlyShown;
    let toolbarHideTimeout;
    let touchX = 0;
    let touchY = 0;

    function pingToolbar() {
        document.querySelector("#slideshowOverlay").style.display = "block";
        global.clearTimeout(toolbarHideTimeout);
        toolbarHideTimeout = global.setTimeout(function hideToolbar() {
            document.querySelector("#slideshowOverlay").style.display = "none";
        }, 3000);
    }

    function displayLoading(target) {
        let isLoading = false;
        let shouldShowLoading = false;
        let iv = setInterval(function() {
            if (shouldShowLoading && !isLoading) {
                target.classList.add("waiting");
                isLoading = true;
            }
        }, 300);
        setTimeout(function () {
            shouldShowLoading = true;
        }, 500)
        return iv
    }

    function undisplayLoading(iv, target) {
        if (target) {
            target.classList.remove("waiting");
        }
        if (iv) {
            clearInterval(iv);
        }
    }

    function showMedia(href, mediaClass) {
        href = href.split("?")[0]; /* clear querystring */
        const contents = document.getElementById("slideshowContents");
        if (mediaClass === "video") {
            contents.innerHTML = `<video controls="true" poster="${href}?thumb" playsinline="true" preload="metadata" autoplay="true">
                <source src="${href}" /></video>`;
        }
        else if (mediaClass === "audio") {
            contents.innerHTML = `<video controls="true" poster="${href}?thumb" playsinline="true" preload="metadata" autoplay="true">
                <source src="${href}" /></video>`;
        }
        else {
            let img = contents.querySelector("img");
            if (!img) { /* Create IMG tag once */
                contents.innerHTML = `<img />`;
                img = contents.querySelector("img");
            }
            img.src = ""; /* Clear the old image */
            const loading = displayLoading(img);
            const offFunc = undisplayLoading.bind(null, loading, img);
            img.addEventListener("load", offFunc);
            img.addEventListener("error", offFunc);
            img.src = href;
        }
        document.getElementById("slideshow").style.display = "block";
    }

    function showSlide(elem) {
        currentlyShown = elem;
        const href = elem.href;
        showMedia(href, elem.parentNode.className);
        history.pushState({
            "url": href,
            "className": elem.parentNode.className,
        }, href, href);
        return false;
    }

    function historyHandle(ev) {
        if (ev.state) {
            showMedia(ev.state.url, ev.state.className);
            document.getElementById("slideshow").style.display = "block";
        }
        else {
            document.getElementById("slideshow").style.display = "none";
            document.getElementById("slideshowContents").innerHTML = "";
        }
    }

    function prevSlide() {
        if (!currentlyShown) {
            return;
        }
        const prev = function findPrev(current) {
            if (!current) {
                return false;
            }
            const prevLi = current.parentNode.previousElementSibling;
            if (!prevLi) {
                return false;
            }
            else if (prevLi.className === 'folder' || prevLi.className === "") {
                return findPrev(prevLi.querySelector("a"));
            }
            return prevLi.querySelector("a");
        }(currentlyShown);
        if (prev) {
            prev.click();
        }
        else {
            cancelSlide();
        }
        return false;
    }

    function nextSlide() {
        if (!currentlyShown) {
            return;
        }
        const next = function findNext(current) {
            if (!current) {
                return false;
            }
            const nextLi = current.parentNode.nextElementSibling;
            if (!nextLi) {
                return false;
            }
            else if (nextLi.className === 'folder' || nextLi.className === "") {
                return findNext(nextLi.querySelector("a"));
            }
            return nextLi.querySelector("a");
        }(currentlyShown);
        if (next) {
            next.click();
        }
        else {
            cancelSlide();
        }
        return false;
    }

    function cancelSlide() {
        document.getElementById("slideshow").style.display = "none";
        document.getElementById("slideshowContents").innerHTML = "";
        if (currentlyShown) {
            currentlyShown.focus();
        }
        currentlyShown = null;
        history.pushState(null, null, '.');
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
            ev.preventDefault();
            return prevSlide();
        case "PageDown":
        case "ArrowDown":
        case "ArrowRight":
            ev.preventDefault();
            return nextSlide();
        case "Space":
            const videoElem = document.querySelector("#slideshow video");
            if (videoElem) { /* space key should pause video */
                ev.preventDefault();
                return videoElem.paused ? videoElem.play() : videoElem.pause();
            } /* not a video so we continue our takeover... */
            case "Tab":
            case "Enter":
                ev.preventDefault();
                return ev.shiftKey ? prevSlide() : nextSlide();
        }
    }

    /* Prevent default event handlers and cancel event bubbling */
    function callOnly(f, e, ...args) {
        e.preventDefault();
        e.stopPropagation();
        f(e, ...args);
    }

    /* Exported so we can load an image in the overlay */
    global.findAndShow = function () {
        const target = global.location.href;
        for (let a of document.querySelectorAll("main a").values()) {
            if (a.href === target) {
                a.click();
                return;
            }
        }
    };

    function touchStartHandle (ev) {
        if (!currentlyShown) {
            return true;
        }
        touchX = ev.changedTouches[0].screenX;
        touchY = ev.changedTouches[0].screenY;
        ev.preventDefault();
        return false;
    }

    function touchEndHandle (ev) {
        if (!currentlyShown) {
            return true;
        }
        let diffX = ev.changedTouches[0].screenX - touchX;
        let diffY = ev.changedTouches[0].screenY - touchY;
        // Ignore vertical swipes
        if (Math.abs(diffY) > 60) {
            return true;
        }
        // Ignore too small side swipes
        if (Math.abs(diffX) < 30) {
            return true;
        }
        if (diffX > 0) {
            prevSlide();
        } else {
            nextSlide();
        }
        ev.preventDefault();
        return false;
    }

    global.addEventListener("load", function init() {
//         const targets = document.querySelectorAll("main a.overlay");
//         if (targets.length === 0) { /* No inline image display is needed */
//             return;
//         }
//         targets.forEach(function (item) {
//             item.addEventListener("click", callOnly.bind(null, showSlide.bind(null, item)));
//         });
        global.addEventListener("keydown", keyHandle);
//         global.addEventListener("popstate", historyHandle);
        global.addEventListener("touchstart", touchStartHandle);
        global.addEventListener("touchend", touchEndHandle);
        /* Mobile browsers seem to react to mousemove on touch */
        document.getElementById("slideshow").addEventListener("mousemove", pingToolbar);
        pingToolbar();
//         document.getElementById("slideshowClose").addEventListener("click", callOnly.bind(null, cancelSlide));
//         document.getElementById("slideshowNext").addEventListener("click", callOnly.bind(null, nextSlide));
//         document.getElementById("slideshowPrev").addEventListener("click", callOnly.bind(null, prevSlide));
    });
}(window));
