GO = go
GOFLAGS = --race

MAKEFILE_DIR:=$(dir $(abspath $(lastword $(MAKEFILE_LIST))))
APPNAME=dev.mtanda.streamdeck.grafana.sdPlugin
BUILDDIR = $(MAKEFILE_DIR)build/$(APPNAME)

.PHONY: build

prepare:
	mkdir -p $(BUILDDIR)
	rm -rf $(BUILDDIR)/*

build: prepare
	cd $(MAKEFILE_DIR) && GOOS=linux GOARCH=amd64 go build -o $(BUILDDIR)/grafana .
	cd $(MAKEFILE_DIR) && GOOS=windows GOARCH=amd64 go build -o $(BUILDDIR)/grafana.exe .
	cp $(MAKEFILE_DIR)*.json $(BUILDDIR)
	cp -a $(MAKEFILE_DIR)ui $(BUILDDIR)
