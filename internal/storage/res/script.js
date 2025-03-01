"use strict";
(function(w) {
    const HideToolbarAfter = 3000;
    let toolbarHideTimeout;
    let touchX = 0;
    let touchY = 0;

    function hideToolbar() {
        const overlay = document.getElementById("slideshowOverlay");
        if (overlay) {
            overlay.style.display = "none";
        }
    }

    function pingToolbar() {
        const overlay = document.getElementById("slideshowOverlay");
        if (overlay) {
            overlay.style.display = "block";
            w.clearTimeout(toolbarHideTimeout);
            toolbarHideTimeout = w.setTimeout(hideToolbar, HideToolbarAfter);
        }
    }

    function prevSlide() {
        const prev = document.getElementById("slideshowPrev");
        if (prev) {
            prev.click();
            return false;
        }
        return true;
    }

    function nextSlide() {
        const next = document.getElementById("slideshowNext");
        if (next) {
            next.click();
            return false;
        }
        return true;
    }
    function parentSlide() {
        let parent = document.getElementById("slideshowParent");
        if (parent) {
            parent.click();
            return false;
        }
        parent = document.getElementById("parentFolder");
        if (parent) {
            parent.click();
            return false;
        }
        return true;
    }

    function keyHandle(ev) {
        const videoElem = document.querySelector("#slideshowContents video");
        switch (ev.code) {
            case "PageUp":
            case "ArrowUp":
            case "ArrowLeft":
                ev.preventDefault();
                if (videoElem && videoElem.currentTime > 1) { /* Skip back 10s */
                    videoElem.currentTime -= 10
                    return false;
                }
                return prevSlide();
            case "PageDown":
            case "ArrowDown":
            case "ArrowRight":
                ev.preventDefault();
                if (videoElem && !videoElem.ended) { /* Skip ahead 10s while playing */
                    videoElem.currentTime += 10
                    return false;
                }
                return nextSlide();
            case "Space":
                if (videoElem) { /* Play/pause with space */
                    ev.preventDefault();
                    if (videoElem.paused) {
                        videoElem.play();
                    } else {
                        videoElem.pause();
                    }
                    return false;
                }
            case "Tab":
            case "Enter":
                ev.preventDefault();
                return ev.shiftKey ? prevSlide() : nextSlide();
            case "Escape":
            case "Backspace":
                return parentSlide();
        }
    }

    function touchStartHandle(ev) {
        if (ev.targetTouches.length > 1) {
            return // Leave multitouch default behaviour unchanged
        }
        touchX = ev.changedTouches[0].screenX;
        touchY = ev.changedTouches[0].screenY;
        ev.preventDefault();
        return false;
    }

    function touchEndHandle(ev) {
        if (ev.targetTouches.length > 1) {
            return
        }
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
        const slideshow = document.getElementById("slideshowContents");
        w.addEventListener("keydown", keyHandle);
        if (slideshow) {
            w.addEventListener("touchstart", touchStartHandle);
            w.addEventListener("touchend", touchEndHandle);
            /* Mobile browsers seem to react to mousemove on touch */
            slideshow.addEventListener("mousemove", pingToolbar);
        }
        hideToolbar();
    });
    w.addEventListener("load", function onloadInit() {
        const slideshow = document.getElementById("slideshowContents");
        if (slideshow) {
            slideshow.classList.add("loaded");
        }
    });
}(window));
