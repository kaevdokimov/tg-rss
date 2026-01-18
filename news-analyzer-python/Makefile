# Makefile for news-analyzer-python

.PHONY: help build test lint clean docker-build docker-run validate-config

# Default target
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  test           - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  lint           - Run linting"
	@echo "  clean          - Clean build artifacts"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  validate-config - Validate configuration"
	@echo "  install        - Install dependencies"
	@echo "  setup          - Full setup (install + validate)"

# Installation
install:
	pip install -r requirements.txt

setup: install validate-config

# Validation
validate-config:
	python validate_config.py

# Building
build:
	python -m py_compile src/**/*.py run_daily.py

# Testing
test:
	python run_tests.py

test-integration:
	python run_integration_tests.py

# Linting (if flake8 is available)
lint:
	-python -m flake8 src/ --max-line-length=120 --ignore=E501,W503 || echo "flake8 not installed, skipping lint"

# Cleaning
clean:
	find . -type d -name __pycache__ -exec rm -rf {} +
	find . -name "*.pyc" -delete
	find . -name "*.pyo" -delete
	rm -rf .pytest_cache
	rm -rf htmlcov
	rm -rf .coverage

# Docker operations
docker-build:
	docker build -t news-analyzer .

docker-run:
	docker run --rm -it \
		-e POSTGRES_HOST=host.docker.internal \
		-e POSTGRES_PORT=5432 \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=password \
		-e POSTGRES_DB=news_bot \
		-v $(PWD)/storage:/app/storage \
		news-analyzer

# Development helpers
run-dev:
	python run_daily.py

run-async:
	USE_ASYNC_PROCESSING=true python run_daily.py

# CI/CD helpers
ci-test: validate-config test
ci-integration: validate-config test-integration
ci-build: clean build lint

# Full development workflow
dev-setup: setup test validate-config
dev-test: test-integration lint