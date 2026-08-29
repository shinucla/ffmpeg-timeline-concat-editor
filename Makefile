.PHONY: samples backend frontend dev build

samples:
	bash scripts/generate-samples.sh

backend:
	cd backend && go run .

frontend:
	cd frontend && npm run dev

dev:
	@echo "Run in two terminals:"
	@echo "  make backend"
	@echo "  make frontend"

build:
	cd backend && go build -o ../bin/server .
	cd frontend && npm run build
