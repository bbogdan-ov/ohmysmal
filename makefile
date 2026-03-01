SERVER_SOURCE := $(shell find . -type f -iname '*.go' -not -iname '*_templ.go')
TEMPL_SOURCE  := $(shell find . -type f -iname '*.templ')

ohmysmal: $(SERVER_SOURCE) $(TEMPL_SOURCE) go.mod go.sum
	go tool templ generate
	go build .
