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
	@git fetch --tags origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || (echo "error: local main is not up to date with origin/main" && exit 1)
	@test "$$(git show-ref --tags --quiet --verify refs/tags/v$(V); echo $$?)" -ne 0 || (echo "error: tag v$(V) already exists" && exit 1)
	git tag -a v$(V) -m "Release v$(V)"
	@echo "Created tag v$(V). Run 'git push origin v$(V)' to trigger release."

release:
	@test -n "$(V)" || (echo "usage: make release V=0.1.0" && exit 1)
	@test "$$(git branch --show-current)" = "main" || (echo "error: not on main branch" && exit 1)
	@git fetch --tags origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" || (echo "error: local main is not up to date with origin/main" && exit 1)
	@gh release view v$(V) >/dev/null 2>&1 && (echo "error: release v$(V) already exists" && exit 1) || true
	@if git show-ref --tags --quiet --verify refs/tags/v$(V); then \
		echo "Tag v$(V) already exists. Skipping tag creation."; \
	else \
		git tag -a v$(V) -m "Release v$(V)"; \
	fi
	@if git ls-remote --exit-code --tags origin refs/tags/v$(V) >/dev/null 2>&1; then \
		echo "Tag v$(V) already exists on origin. Skipping push."; \
	else \
		git push origin v$(V); \
	fi
