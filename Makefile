.PHONY: git build run git

git:
	@git add .
	@git commit -m "$(or $(msg),commit)" || true
	@git push	

build:
	@go build -o bin/api ./cmd/api

run: build
	@./bin/api	

