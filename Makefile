.PHONY: build test lint lint-fix release-dry-run tag release

build:
	go build -o xr .

test:
	go test ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

release-dry-run:
	goreleaser release --snapshot --clean

tag:
	@test -n "$(V)" || (echo "usage: make tag V=0.1.0" && exit 1)
	@test "$$(git branch --show-current)" = "main" || (echo "error: not on main branch" && exit 1)
	@git fetch origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || (echo "error: local main is not up to date with origin/main" && exit 1)
	git tag -a v$(V) -m "Release v$(V)"
	@echo "Created tag v$(V). Run 'git push origin v$(V)' to trigger release."

release:
	@test -n "$(V)" || (echo "usage: make release V=0.1.0" && exit 1)
	@test "$$(git branch --show-current)" = "main" || (echo "error: not on main branch" && exit 1)
	@git fetch origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || (echo "error: local main is not up to date with origin/main" && exit 1)
	git tag -a v$(V) -m "Release v$(V)"
	git push origin v$(V)
