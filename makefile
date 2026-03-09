SERVER_SOURCE := $(shell find . -type f -iname '*.go' -not -iname '*_templ.go')
TEMPL_SOURCE  := $(shell find . -type f -iname '*.templ')
COMPILER_SOURCE := $(shell find . -type f -iname '*.rs')

.PHONY: all
all: ohmysmal

ohmysmal: $(SERVER_SOURCE) $(TEMPL_SOURCE) go.mod go.sum static/wasm/compiler.wasm
	go tool templ generate
	go build .

static/wasm/compiler.wasm: $(COMPILER_SOURCE)
	cd compiler && cargo build --release --target=wasm32-unknown-unknown
	cp compiler/target/wasm32-unknown-unknown/release/compiler.wasm static/wasm/compiler.wasm
