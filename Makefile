build: cleanup build_app_linux_amd64 build_app_linux_arm64 build_app_windows_amd64 build_app_windows_arm64 docker

build_dev: cleanup build_app_linux_amd64 docker_dev

build_app_linux_amd64:
	@echo "Building for Linux AMD64..."
	go env -w GOOS=linux GOARCH=amd64
	go build -o out/pbp-tunnel ./cmd/pbp-tunnel
	go env -u GOOS GOARCH

build_app_linux_arm64:
	@echo "Building for Linux ARM64..."
	go env -w GOOS=linux GOARCH=arm64
	go build -o out/pbp-tunnel ./cmd/pbp-tunnel
	go env -u GOOS GOARCH

build_app_windows_amd64:
	@echo "Building for Windows AMD64..."
	go env -w GOOS=windows GOARCH=amd64
	go build -o out/pbp-tunnel.exe ./cmd/pbp-tunnel
	go env -u GOOS GOARCH

build_app_windows_arm64:
	@echo "Building for Windows ARM64..."
	go env -w GOOS=windows GOARCH=arm64
	go build -o out/pbp-tunnel.exe ./cmd/pbp-tunnel
	go env -u GOOS GOARCH

docker:
	docker build . --build-arg ARTIFACT_NAME=out/pbp-tunnel -t pbp-tunnel:latest

docker_dev:
	docker build . --build-arg ARTIFACT_NAME=out/pbp-tunnel -t pbp-tunnel:dev

cleanup:
	rm -rf out/pbp-tunnel*

default: build
