.PHONY: samples backend frontend run.all dev build

samples:
	bash scripts/generate-samples.sh

backend:
	cd backend && go run .

frontend:
	cd frontend && npm run dev

# Launch backend and frontend together; Ctrl+C stops both.
run.all:
	@echo "Starting backend (:8000) and frontend (:5173) — Ctrl+C to stop both"
	@trap 'kill 0' INT TERM EXIT; \
	( cd backend && go run . ) & \
	( cd frontend && npm run dev ) & \
	wait

dev:
	@echo "Run in two terminals:"
	@echo "  make backend"
	@echo "  make frontend"

build:
	cd backend && go build -o ../bin/server .
	cd frontend && npm run build
