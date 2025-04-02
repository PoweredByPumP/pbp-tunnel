build_dev:
	go build -o pbp-tunnel .
	docker build . -t pbp-tunnel:dev

default: build_dev
