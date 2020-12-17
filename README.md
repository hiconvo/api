# Convo API

[![CircleCI](https://img.shields.io/circleci/build/github/hiconvo/api?label=circleci)](https://circleci.com/gh/hiconvo/api) [![codecov](https://img.shields.io/codecov/c/gh/hiconvo/api)](https://codecov.io/gh/hiconvo/api) [![goreportcard](https://goreportcard.com/badge/github.com/hiconvo/api)](https://goreportcard.com/badge/github.com/hiconvo/api)

The repo holds the source code for Convo's RESTful API. Learn more about Convo at [convo.events](https://convo.events).

## Development

We use docker based development. In order to run the project locally, you need to create an `.env` file and place it at the root of the project. The `.env` file should contain a Google Maps API key, Sendgrid API key, and a Stream API key and secret. It should look something like this:

```
GOOGLE_MAPS_API_KEY=<YOUR API KEY>
SENDGRID_API_KEY=<YOUR API KEY>
STREAM_API_KEY=<YOUR API KEY>
STREAM_API_SECRET=<YOUR API SECRET>
```

If you don't include this file, the app will panic during startup.

After your `.env` file is ready, all you need to do is run `docker-compose up`. The source code is shared between your machine and the docker container via a volume. The default command runs [`air`](https://github.com/cosmtrek/air), a file watcher that automatically compiles the code and restarts the server when the source changes. By default, the server listens on port `:8080`.

### Running Tests

Run `docker ps` to get the ID of the container running the API. Then run

```
docker exec -it <CONTAINER ID> go test ./...
```

Be mindful that this command will *wipe everything from the database*. There is probably a better way of doing this, but I haven't taken the time to improve this yet.

## Maintenance Commands

```
# Update datastore indexes
gcloud datastore indexes create index.yaml

# Delete unused indexes
gcloud datastore cleanup-indexes index.yaml

# Update cron jobs
gcloud app deploy cron.yaml
```

## One-Off Commands

```
# Get credentials to connect to the production database. [DANGEROUS]
gcloud auth application-default login

# Run the command. Example:
go run cmd/migrate-message-timestamps-and-photos/main.go --dry-run

# Clean up. [ALWAYS REMEMBER]
gcloud auth application-default revoke
```

## Architecture

![Architecture](architecture.jpg)
