![Benthos](icon.png "Benthos")

[![godoc for benthosdev/benthos][godoc-badge]][godoc-url]
[![Build Status][actions-badge]][actions-url]
[![Discord invite][discord-badge]][discord-url]
[![Docs site][website-badge]][website-url]

Benthos is a high performance and resilient stream processor, able to connect various [sources][inputs] and [sinks][outputs] in a range of brokering patterns and perform [hydration, enrichments, transformations and filters][processors] on payloads.

It comes with a [powerful mapping language][bloblang-about], is easy to deploy and monitor, and ready to drop into your pipeline either as a static binary, docker image, or [serverless function][serverless], making it cloud native as heck.

Benthos is fully declarative, with stream pipelines defined in a single config file, allowing you to specify connectors and a list of processing stages:

```yaml
input:
  gcp_pubsub:
    project: foo
    subscription: bar

pipeline:
  processors:
    - bloblang: |
        root.message = this
        root.meta.link_count = this.links.length()
        root.user.age = this.user.age.number()

output:
  redis_streams:
    url: tcp://TODO:6379
    stream: baz
    max_in_flight: 20
```

### Delivery Guarantees

Yep, we got 'em. Benthos implements transaction based resiliency with back pressure. When connecting to at-least-once sources and sinks it guarantees at-least-once delivery without needing to persist messages during transit.

## Supported Sources & Sinks

Apache Pulsar, AWS (DynamoDB, Kinesis, S3, SQS, SNS), Azure (Blob storage, Queue storage, Table storage), Cassandra, Elasticsearch, File, GCP (Pub/Sub, Cloud storage), HDFS, HTTP (server and client, including websockets), Kafka, Memcached, MQTT, Nanomsg, NATS, NATS JetStream, NATS Streaming, NSQ, AMQP 0.91 (RabbitMQ), AMQP 1, Redis (streams, list, pubsub, hashes), MongoDB, SQL (MySQL, PostgreSQL, Clickhouse, MSSQL), Stdin/Stdout, TCP & UDP, sockets and ZMQ4.

Connectors are being added constantly, if something you want is missing then [open an issue](https://github.com/benthosdev/benthos/issues/new).

## Documentation

If you want to dive fully into Benthos then don't waste your time in this dump, check out the [documentation site][general-docs].

For guidance on how to configure more advanced stream processing concepts such as stream joins, enrichment workflows, etc, check out the [cookbooks section.][cookbooks]

For guidance on building your own custom plugins in Go check out [the public APIs.][godoc-url]

## Install

Grab a binary for your OS from [here.][releases] Or use this script:

```shell
curl -Lsf https://sh.benthos.dev | bash
```

Or pull the docker image:

```shell
docker pull jeffail/benthos
```

Benthos can also be installed via Homebrew:

```shell
brew install benthos
```

For more information check out the [getting started guide][getting-started].

## Run

```shell
benthos -c ./config.yaml
```

Or, with docker:

```shell
# Using a config file
docker run --rm -v /path/to/your/config.yaml:/benthos.yaml jeffail/benthos

# Using a series of -s flags
docker run --rm -p 4195:4195 jeffail/benthos \
  -s "input.type=http_server" \
  -s "output.type=kafka" \
  -s "output.kafka.addresses=kafka-server:9092" \
  -s "output.kafka.topic=benthos_topic"
```

## Monitoring

### Health Checks

Benthos serves two HTTP endpoints for health checks:
- `/ping` can be used as a liveness probe as it always returns a 200.
- `/ready` can be used as a readiness probe as it serves a 200 only when both the input and output are connected, otherwise a 503 is returned.

### Metrics

Benthos [exposes lots of metrics][metrics] either to Statsd, Prometheus or for debugging purposes an HTTP endpoint that returns a JSON formatted object.

### Tracing

Benthos also [emits tracing events][tracers] to a tracer of your choice (currently only [Jaeger][jaeger] is supported) which can be used to visualise the processors within a pipeline.

## Configuration

Benthos provides lots of tools for making configuration discovery, debugging and organisation easy. You can [read about them here][config-doc].

## Build

Build with Go (1.16 or later):

```shell
git clone git@github.com:benthosdev/benthos
cd benthos
make
```

## Lint

Benthos uses [golangci-lint][golangci-lint] for linting, which you can install with:

```shell
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

And then run it with `make lint`.

### Plugins

It's pretty easy to write your own custom plugins for Benthos in Go, for information check out [the API docs][godoc-url], and for inspiration there's an [example repo][plugin-repo] demonstrating a variety of plugin implementations.

### Docker Builds

There's a multi-stage `Dockerfile` for creating a Benthos docker image which results in a minimal image from scratch. You can build it with:

```shell
make docker
```

Then use the image:

```shell
docker run --rm \
	-v /path/to/your/benthos.yaml:/config.yaml \
	-v /tmp/data:/data \
	-p 4195:4195 \
	benthos -c /config.yaml
```

## Contributing

Contributions are welcome, please [read the guidelines](CONTRIBUTING.md), come and chat (links are on the [community page][community]), and watch your back.

[inputs]: https://www.benthos.dev/docs/components/inputs/about
[processors]: https://www.benthos.dev/docs/components/processors/about
[outputs]: https://www.benthos.dev/docs/components/outputs/about
[metrics]: https://www.benthos.dev/docs/components/metrics/about
[tracers]: https://www.benthos.dev/docs/components/tracers/about
[config-interp]: https://www.benthos.dev/docs/configuration/interpolation
[streams-api]: https://www.benthos.dev/docs/guides/streams_mode/streams_api
[streams-mode]: https://www.benthos.dev/docs/guides/streams_mode/about
[general-docs]: https://www.benthos.dev/docs/about
[bloblang-about]: https://www.benthos.dev/docs/guides/bloblang/about
[config-doc]: https://www.benthos.dev/docs/configuration/about
[serverless]: https://www.benthos.dev/docs/guides/serverless/about
[cookbooks]: https://www.benthos.dev/cookbooks
[releases]: https://github.com/benthosdev/benthos/releases
[plugin-repo]: https://github.com/benthosdev/benthos-plugin-example
[getting-started]: https://www.benthos.dev/docs/guides/getting_started

[godoc-badge]: https://pkg.go.dev/badge/github.com/benthosdev/benthos/v4/public
[godoc-url]: https://pkg.go.dev/github.com/benthosdev/benthos/v4/public
[actions-badge]: https://github.com/benthosdev/benthos/actions/workflows/test.yml/badge.svg
[actions-url]: https://github.com/benthosdev/benthos/actions/workflows/test.yml
[discord-badge]: https://img.shields.io/discord/746368194196799589
[discord-url]: https://discord.com/invite/6VaWjzP
[website-badge]: https://img.shields.io/badge/Docs-Learn%20more-ffc7c7
[website-url]: https://www.benthos.dev

[community]: https://www.benthos.dev/community

[golangci-lint]: https://golangci-lint.run/
[jaeger]: https://www.jaegertracing.io/
