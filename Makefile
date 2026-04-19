.PHONY: build/otk-gitsync
build/otk-gitsync:
	mise exec -- go build -o $@ ./cmd/otk-gitsync

.PHONY: go-mod-download
go-mod-download:
	mise exec -- go mod download
	cd testing && mise exec -- go mod download

.PHONY: go-mod-tidy
go-mod-tidy:
	mise exec -- go mod tidy
	cd testing && mise exec -- go mod tidy

.PHONY: go-update-deps
go-update-deps:
	mise exec -- go get -u ./...
	cd testing && mise exec -- go get -u ./...

.PHONY: test
test:
	mise exec -- go test -race ./...

.PHONY: testing-init-podman
testing-init-podman:
	systemctl --user start podman.socket

.PHONY: test-integration-podman
test-integration-podman:
	cd testing && \
	DOCKER_HOST="unix://$$(podman info --format '{{.Host.RemoteSocket.Path}}')" \
	TESTCONTAINERS_RYUK_DISABLED=true \
	mise exec -- go test -race -v ./...

.PHONY: lint
lint:
	mise exec -- golangci-lint run

.PHONY: lint-fix
lint-fix:
	mise exec -- golangci-lint run --fix

.PHONY: format
format:
	mise exec -- golangci-lint fmt
