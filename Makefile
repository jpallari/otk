.PHONY: build/otk-gitsync
build/otk-gitsync:
	go build -o $@ ./cmd/otk-gitsync

.PHONY: go-mod-download
go-mod-download:
	go mod download
	cd testing && go mod download

.PHONY: go-mod-tidy
go-mod-tidy:
	go mod tidy
	cd testing && go mod tidy

.PHONY: go-update-deps
go-update-deps:
	go get -u ./...
	cd testing && go get -u ./...

.PHONY: test
test:
	go test -race ./...

.PHONY: testing-init-podman
testing-init-podman:
	systemctl --user start podman.socket

.PHONY: test-integration-podman
test-integration-podman:
	cd testing && \
	DOCKER_HOST="unix://$$(podman info --format '{{.Host.RemoteSocket.Path}}')" \
	TESTCONTAINERS_RYUK_DISABLED=true \
	go test -race -v ./...
