init: docker-down-clear docker-pull docker-build up
up: docker-up
down: docker-down
restart: down up

############################
# DOCKER
############################
docker-up:
	docker-compose --env-file .env.local up -d
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
	docker exec -it news_bot sh