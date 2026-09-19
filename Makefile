# Mosaic - Native Gotk4 Music Player & Widget Library
# Compiles C/C++ via Zig CC using CGO

CC       := zig cc
CXX      := zig c++
BIN_APP  := mosaic
BIN_LIB  := widget-library

.PHONY: all build run gallery list clean help

all: build

## build: Compiles the Mosaic application from cmd/mosaic/ using Zig CC
build:
	CGO_ENABLED=1 CC="$(CC)" CXX="$(CXX)" go build -v -o $(BIN_APP) ./cmd/mosaic

## run: Runs the Mosaic application
run: build
	./$(BIN_APP)

## gallery: Builds and launches the interactive Gotk4 Widget Gallery UI
gallery:
	CGO_ENABLED=1 CC="$(CC)" CXX="$(CXX)" go build -v -o $(BIN_LIB) ./cmd/widget-library
	./$(BIN_LIB)

## list: Lists all registered categories and widgets in the library
list:
	CGO_ENABLED=1 CC="$(CC)" CXX="$(CXX)" go build -v -o $(BIN_LIB) ./cmd/widget-library
	./$(BIN_LIB) --list

## clean: Removes compiled binaries
clean:
	rm -f $(BIN_APP) $(BIN_LIB) moosic music-player

## help: Shows available targets
help:
	@echo "Available make targets:"
	@echo "  make build    - Compile Mosaic from ./cmd/mosaic/ using 'zig cc'"
	@echo "  make run      - Build and run Mosaic"
	@echo "  make gallery  - Build and run the Gotk4 Widget Gallery UI"
	@echo "  make list     - Print all available categories and widgets in the gallery"
	@echo "  make clean    - Remove compiled binaries"
