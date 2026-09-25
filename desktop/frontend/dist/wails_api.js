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
                window.location.replace("/setup.html");
            }
        } catch (e) {
            reveal();
        }
    });
})();
