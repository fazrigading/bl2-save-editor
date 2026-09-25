// Wails first-run check, mirroring app.py's index route: whenever config.json
// is missing (or save_dir is empty), show the setup wizard instead of the
// editor. The transport (api()) lives in app.js itself and talks to the Go
// bridge via window.go.bridge.Bridge.
(function () {
    // Page stays hidden until we know which screen to show (Python renders
    // either setup.html or index.html server-side; we emulate that here).
    function reveal() {
        document.documentElement.classList.remove("boot-wait");
    }

    function backend() {
        return window.go && window.go.bridge ? window.go.bridge.Bridge : null;
    }

    document.addEventListener("DOMContentLoaded", async function () {
        if (window.location.pathname.indexOf("setup") !== -1) {
            reveal();
            return;
        }
        // Wait for the Wails runtime bindings (usually immediate, but never
        // assume — fall back to showing the editor after 5s).
        var b = null;
        for (var i = 0; i < 100; i++) {
            b = backend();
            if (b) break;
            await new Promise(function (r) { setTimeout(r, 50); });
        }
        if (!b) {
            reveal();
            return;
        }
        try {
            var raw = await b.Invoke("GET", "/api/configured", null);
            var res = JSON.parse(raw);
            if (res.configured) {
                reveal();
            } else {
                // /setup/ is a directory page: the Wails runtime (window.go)
                // is only injected into paths ending in "/" or /index.html.
                window.location.replace("/setup/");
            }
        } catch (e) {
            reveal();
        }
    });

    // Native menu → frontend navigation (Settings → Re-run Setup Wizard).
    document.addEventListener("DOMContentLoaded", function () {
        if (!window.runtime || !window.runtime.EventsOn) return;
        window.runtime.EventsOn("app:navigate", function (path) {
            if (path === "/" || path === "/setup/") window.location.replace(path);
        });
    });

    // Gibbed auto-download toasts in the editor (the setup screen has its
    // own listeners and doesn't load this file).
    document.addEventListener("DOMContentLoaded", function () {
        if (!window.runtime || !window.runtime.EventsOn) return;
        window.runtime.EventsOn("gibbed:done", function () {
            if (typeof toast === "function") toast("Gibbed data installed — item database enabled", "success");
        });
        window.runtime.EventsOn("gibbed:error", function (d) {
            if (typeof toast === "function") toast("Gibbed data download failed: " + d.error, "warning");
        });
    });
})();
