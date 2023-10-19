"use strict";
(function (w) {
    const HideToolbarAfter = 3000;
    let toolbarHideTimeout;
    let touchX = 0;
    let touchY = 0;

    function hideToolbar() {
        document.getElementById("slideshowOverlay").style.display = "none";
    }

    function pingToolbar() {
        document.getElementById("slideshowOverlay").style.display = "block";
        w.clearTimeout(toolbarHideTimeout);
        toolbarHideTimeout = w.setTimeout(hideToolbar, HideToolbarAfter);
    }

    function prevSlide() {
        const prev = document.getElementById("slideshowPrev");
        if (prev) {
            prev.click();
        }
        return false;
    }

    function nextSlide() {
        const next = document.getElementById("slideshowNext");
        if (next) {
            next.click();
        }
        return false;
    }

    function keyHandle(ev) {
        switch (ev.code) {
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

    function touchStartHandle (ev) {
        touchX = ev.changedTouches[0].screenX;
        touchY = ev.changedTouches[0].screenY;
        ev.preventDefault();
        return false;
    }

    function touchEndHandle (ev) {
        let diffX = ev.changedTouches[0].screenX - touchX;
        let diffY = ev.changedTouches[0].screenY - touchY;
        /* Ignore vertical swipes */
        if (Math.abs(diffY) > 60) {
            return true;
        }
        /* Ignore too small side swipes */
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

    w.addEventListener("DOMContentLoaded", function init() {
        w.addEventListener("keydown", keyHandle);
        w.addEventListener("touchstart", touchStartHandle);
        w.addEventListener("touchend", touchEndHandle);
        /* Mobile browsers seem to react to mousemove on touch */
        document.getElementById("slideshowContents").addEventListener("mousemove", pingToolbar);
        hideToolbar();
    });
    w.addEventListener("load", function onloadInit() {
        document.getElementById("slideshowContents").classList.add("loaded");
    });
}(window));
