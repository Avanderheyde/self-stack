.PHONY: build test run clean dashboard audit

audit:
	scripts/secret-audit.sh

dashboard:
	cd dashboard && npm run build
	rm -rf internal/dashboard/dist
	cp -r dashboard/dist internal/dashboard/dist

build: dashboard
	go build -o selfstack .

test:
	go test ./... -v

run: build
	./selfstack serve

clean:
	rm -f selfstack
	rm -rf internal/dashboard/dist
