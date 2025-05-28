# Syslog Webhook to Kafka

A Go application that collects logs from syslog and webhook sources and forwards them to Kafka.

## Features

- Multiple syslog servers support (UDP/TCP)
- Multiple webhook endpoints support (HTTP/HTTPS)
- Multiple Kafka instances support
- Dynamic message key extraction from JSON data
- JSON format validation
- Graceful shutdown support

## Configuration

The application is configured using a YAML file. Here's an example configuration:

```yaml
kafka:
  - id: kafka1
    brokers:
      - localhost:9092
    topic: logs
    key:
      field: timestamp
      type: timestamp

syslog:
  - listen: 0.0.0.0:514
    format: json
    protocol: udp
    kafka_id: kafka1

webhook:
  - listen: http://0.0.0.0:8080
    path: /webhook
    kafka_id: kafka1
    tls:
      enabled: false
```

### Configuration Details

#### Kafka Configuration
- `id`: Unique identifier for the Kafka instance
- `brokers`: List of Kafka broker addresses
- `topic`: Kafka topic to send messages to
- `key`: Optional message key configuration
  - `field`: JSON field to use as message key
  - `type`: Key type (string, number, timestamp)

#### Syslog Configuration
- `listen`: Address to listen on (e.g., "0.0.0.0:514")
- `format`: Message format (e.g., "json")
- `protocol`: Transport protocol (udp/tcp)
- `kafka_id`: ID of the Kafka instance to use

#### Webhook Configuration
- `listen`: HTTP(S) address to listen on
- `path`: Webhook endpoint path
- `kafka_id`: ID of the Kafka instance to use
- `tls`: Optional TLS configuration for HTTPS
  - `enabled`: Enable TLS
  - `cert_file`: Path to certificate file
  - `key_file`: Path to private key file

## Building and Running

1. Build the application:
```bash
go build
```

2. Create a configuration file (config.yaml)

3. Run the application:
```bash
./syslog_webhook_to_kafka
```

## Testing

Run the test suite:
```bash
go test ./...
```

## License

MIT License

Copyright (c) 2024

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
