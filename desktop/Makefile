# Build targets. Linux builds use webkit2gtk-4.1 (webkit2_41 build tag);
# install the dev packages first: webkit2gtk4.1-devel (Fedora) or
# libgtk-3-dev libwebkit2gtk-4.1-dev (Debian/Ubuntu).

.PHONY: build dev patch

build:
	wails build -clean -tags webkit2_41

dev:
	wails dev -tags webkit2_41

patch:
	go build -tags webkit2_41 -o build/bin/bl2patch ./cmd/bl2patch

build-native:
	go build -o build/bin/bl2native ./cmd/bl2native

dev-native:
	go run ./cmd/bl2native
