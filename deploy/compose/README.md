# Local development environment (Docker Compose)

Runs the shared infrastructure for JobTrack on your laptop:

| Container | Purpose | From your laptop |
|---|---|---|
| `postgres` | One PostgreSQL server, one database and user per service | `localhost:5432` |
| `rabbitmq` | Message broker with management UI | AMQP `localhost:5672`, UI http://localhost:15672 |
| `mailpit` | Test email inbox (no real emails are sent) | SMTP `localhost:1025`, inbox http://localhost:8025 |

Inside the Docker network, services use the hostnames `postgres`, `rabbitmq` and `mailpit`.

## Requirements

- Docker with Docker Compose v2 (`docker compose version`)
- On Windows, run the commands in WSL (Ubuntu)
- About 2 GB free RAM

## First-time setup

Run all commands from the repo root.

```bash
cp deploy/compose/.env.example deploy/compose/.env
```

Edit `deploy/compose/.env` and replace every `change-me` with your own local passwords before the first start. `.env` is ignored by Git. Never commit it.

## Start

```bash
docker compose -f deploy/compose/docker-compose.yml up -d
docker compose -f deploy/compose/docker-compose.yml ps
```

Wait until `postgres` and `rabbitmq` show `(healthy)`. The first start downloads the images and can take several minutes.

## Check it works

```bash
bash deploy/compose/smoke-test.sh
```

It should end with `All checks passed.` It checks that all containers are healthy, that the 20 service databases exist, that each service user can log in only to its own database, and that RabbitMQ and Mailpit respond.

## Stop

```bash
docker compose -f deploy/compose/docker-compose.yml down
```

Your data is kept in Docker volumes and is still there on the next start.

## Reset (delete all local data)

```bash
docker compose -f deploy/compose/docker-compose.yml down -v
docker compose -f deploy/compose/docker-compose.yml up -d
```

`-v` deletes the Postgres and RabbitMQ volumes. On the next start the init script creates all 20 databases and users again.

You must reset after changing `POSTGRES_PASSWORD` or `SERVICE_DB_PASSWORD` in `.env`. The init script only runs on an empty volume, so Postgres keeps the old passwords until you reset.

## Useful commands

```bash
# Logs of one container
docker compose -f deploy/compose/docker-compose.yml logs -f postgres

# Open psql as the admin user
docker compose -f deploy/compose/docker-compose.yml exec postgres psql -U postgres

# List the service databases
docker compose -f deploy/compose/docker-compose.yml exec postgres psql -U postgres -c "\l"
```

## Adding a service

`docker-compose.yml` has a commented example block for `auth`. Copy it, uncomment it, and replace `auth` with your service name:

- kebab-case for the block name and `SERVICE_NAME` (for example `application-status`)
- snake_case in `DATABASE_URL` for the user and database (for example `application_status`)

Do not publish port 9090 (gRPC) to the host (see AGENTS.md). Services reach each other through the Docker network.

Each service's database and user already exist. If you add a new service that is not in the list in `postgres-init/01-create-service-databases.sh`, add it there and in `smoke-test.sh`, then reset.

## Troubleshooting

- `password authentication failed`: `.env` was changed after the first start. Reset (see above).
- `required variable ... is missing a value: set in .env`: you have not created `deploy/compose/.env`. See First-time setup.
- `port is already allocated`: another program uses that port, often a local Postgres on 5432. Stop it, or change the left side of the port mapping in your local copy only.
- `/bin/bash^M: bad interpreter` in the Postgres logs: the init script has Windows line endings. The repo's `.gitattributes` prevents this; if your editor changed it, save the file with LF line endings and reset.
