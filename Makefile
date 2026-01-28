VERSION := $(shell cat VERSION 2>/dev/null || echo "0.0.0")

.PHONY: help build release-patch release-minor release-major

help:
	@echo "Available commands:"
	@echo "  make build          - Build locally"
	@echo "  make release-patch  - Release patch version (x.x.X+1)"
	@echo "  make release-minor  - Release minor version (x.X+1.0)"
	@echo "  make release-major  - Release major version (X+1.0.0)"

build:
	go build -o statusline$(shell go env GOEXE) ./cmd/statusline

release-patch:
	@NEW_VERSION=$$(echo $(VERSION) | awk -F. '{print $$1"."$$2".".$$3+1}'); \
	echo "$$NEW_VERSION" > VERSION; \
	git add VERSION; \
	git commit -m "chore: release v$$NEW_VERSION"; \
	git tag "v$$NEW_VERSION"; \
	git push origin main; \
	git push origin "v$$NEW_VERSION"; \
	echo "Released v$$NEW_VERSION successfully!"

release-minor:
	@NEW_VERSION=$$(echo $(VERSION) | awk -F. '{print $$1"."$$2+1".0"}'); \
	echo "$$NEW_VERSION" > VERSION; \
	git add VERSION; \
	git commit -m "chore: release v$$NEW_VERSION"; \
	git tag "v$$NEW_VERSION"; \
	git push origin main; \
	git push origin "v$$NEW_VERSION"; \
	echo "Released v$$NEW_VERSION successfully!"

release-major:
	@NEW_VERSION=$$(echo $(VERSION) | awk -F. '{print $$1+1".0.0"}'); \
	echo "$$NEW_VERSION" > VERSION; \
	git add VERSION; \
	git commit -m "chore: release v$$NEW_VERSION"; \
	git tag "v$$NEW_VERSION"; \
	git push origin main; \
	git push origin "v$$NEW_VERSION"; \
	echo "Released v$$NEW_VERSION successfully!"
