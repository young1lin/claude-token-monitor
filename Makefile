VERSION := $(shell cat VERSION 2>/dev/null || echo "0.0.0")

.PHONY: help build release release-patch release-minor release-major

help:
	@echo "Available commands:"
	@echo "  make build          - Build locally"
	@echo "  make release        - Release new version (interactive)"
	@echo "  make release-patch  - Release patch version (x.x.X+1)"
	@echo "  make release-minor  - Release minor version (x.X+1.0)"
	@echo "  make release-major  - Release major version (X+1.0.0)"

build:
	go build -o statusline$(shell go env GOEXE) ./cmd/statusline

release:
	@echo "Current version: $(VERSION)"
	@read -p "Enter new version: " NEW_VERSION; \n	if [ -z "$$NEW_VERSION" ]; then echo "Error: version cannot be empty"; exit 1; fi; \n	echo "$$NEW_VERSION" > VERSION; \n	git add VERSION; \n	git commit -m "chore: release v$$NEW_VERSION"; \n	git tag "v$$NEW_VERSION"; \n	git push origin main; \n	git push origin "v$$NEW_VERSION"; \n	echo ""; \n	echo "Released v$$NEW_VERSION successfully\!"; \n	echo "GitHub Actions building: https://github.com/young1lin/claude-token-monitor/actions"

release-patch:
	@NEW_VERSION=$$(echo $(VERSION) | awk -F. '{print $$1"."$$2"."$$3+1}'); \n	echo "$$NEW_VERSION" > VERSION; \n	git add VERSION; \n	git commit -m "chore: release v$$NEW_VERSION"; \n	git tag "v$$NEW_VERSION"; \n	git push origin main; \n	git push origin "v$$NEW_VERSION"; \n	echo "Released v$$NEW_VERSION successfully\!"

release-minor:
	@NEW_VERSION=$$(echo $(VERSION) | awk -F. '{print $$1"."$$2+1".0"}'); \n	echo "$$NEW_VERSION" > VERSION; \n	git add VERSION; \n	git commit -m "chore: release v$$NEW_VERSION"; \n	git tag "v$$NEW_VERSION"; \n	git push origin main; \n	git push origin "v$$NEW_VERSION"; \n	echo "Released v$$NEW_VERSION successfully\!"

release-major:
	@NEW_VERSION=$$(echo $(VERSION) | awk -F. '{print $$1+1".0.0"}'); \n	echo "$$NEW_VERSION" > VERSION; \n	git add VERSION; \n	git commit -m "chore: release v$$NEW_VERSION"; \n	git tag "v$$NEW_VERSION"; \n	git push origin main; \n	git push origin "v$$NEW_VERSION"; \n	echo "Released v$$NEW_VERSION successfully\!"
