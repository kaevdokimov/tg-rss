init: docker-down-clear docker-pull docker-build up
up: docker-up
down: docker-down
restart: down up

############################
# DOCKER
############################
docker-up:
	docker-compose --env-file .env up -d
docker-down:
	docker-compose down
# Will delete all volumes
docker-down-clear:
	docker-compose down -v --remove-orphans
docker-pull:
	docker-compose pull
docker-build:
	docker-compose build --pull
logs:
	docker-compose logs -f

console:
	docker exec -it bot sh

############################
# NEWS ANALYZER (Python)
############################
analyzer-logs:
	docker-compose logs -f news-analyzer
analyzer-console:
	docker exec -it news-analyzer bash
analyzer-run:
	docker exec -it news-analyzer python run_daily.py
analyzer-test:
	docker exec -it news-analyzer python test_connection.py
analyzer-build:
	docker-compose build news-analyzer
analyzer-restart:
	docker-compose restart news-analyzer
analyzer-stop:
	docker-compose stop news-analyzer
analyzer-start:
	docker-compose start news-analyzer