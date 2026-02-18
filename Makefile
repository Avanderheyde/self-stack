.PHONY: build test run clean

build:
	go build -o selfstack .

test:
	go test ./... -v

run: build
	./selfstack

clean:
	rm -f selfstack
