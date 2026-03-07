.PHONY: build run test clean

APP := hexone
CMD := ./cmd/hexone

ifeq ($(OS),Windows_NT)
BIN := $(APP).exe
BUILD_FLAGS := -ldflags="-H windowsgui"
else
BIN := $(APP)
BUILD_FLAGS :=
endif

build:
	go build $(BUILD_FLAGS) -o $(BIN) $(CMD)

run:
	go run $(CMD)

test:
	go test ./...

clean:
	$(RM) $(BIN)
