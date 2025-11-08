.PHONY: all build install clean

SLAVESIM_BINARY=slavesim
MASTER_BINARY=master
SLAVESIM_PATH=./cmd/slavesim
MASTER_PATH=./cmd/master

all: install

build:
	go build -o $(SLAVESIM_BINARY) $(SLAVESIM_PATH)
	go build -o $(MASTER_BINARY) $(MASTER_PATH)

install:
	go install $(SLAVESIM_PATH)
	go install $(MASTER_PATH)

clean:
	go clean
	rm -f $(SLAVESIM_BINARY) $(MASTER_BINARY)
