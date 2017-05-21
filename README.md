# Event Store

The service provides a simple HTTP API which allows
committing events to a persisted log. This is the command service.
Consuming the event log should be managed through a different query
service. No such query service has been built at time of writing.

## Getting Started

This API should work with minimal setup.

1. Install gin
  * `go get github.com/codegangsta/gin`
2. Run with gin (ensure your pwd is this cloned repo)
  * `gin`
3. Test the server
  * `curl http://127.0.0.1:3000`

## Architecture

Events are persisted to Amazon DynamoDB. All streams are persisted to a
single DynamoDB table. The current implementation _does not_ allow for
running multiple instances of the service in parallel - there can only
be a single writer. Horizontal scalability is an anticipated future
feature.

There is no locking mechanism implemented to enforce only a single
writer to a given DynamoDB table. You must enforce this yourself or risk
corruption to the store.

## API

### POST /:streamId

This request will append the given event as the next version on the
stream.

#### Request Body

```json
{
    "type": "AmountDeposited",
    "data": "eyJhbW91bnQiOjEwMDAwLCJ0aW1lc3RhbXAiOiIyMDE3LTA1LTIwVDIyOjMwOjI2WiJ9"
}
```

`type` represents the event type. The value is entirely up to the client
and indicates to all consumers how the `data` value should be
deserialized.

`date` should be a Base64 encoded value for the event. The contained
data format is entirely up to the client.

#### Response Codes

- 200 OK
  - The event was successfully committed to the log.
- 400 BAD_REQUEST
  - There was an issue parsing the request - likely a formatting error.

### POST /:streamId/:version

This request will append the given event as the next version on the stream
if and only if the specified version is the next sequence in the stream.

#### Request Body

```json
{
    "type": "AmountDeposited",
    "data": "eyJhbW91bnQiOjEwMDAwLCJ0aW1lc3RhbXAiOiIyMDE3LTA1LTIwVDIyOjMwOjI2WiJ9"
}
```

`type` represents the event type. The value is entirely up to the client
and indicates to all consumers how the `data` value should be
deserialized.

`date` should be a Base64 encoded value for the event. The contained
data format is entirely up to the client.

#### Response Codes

- 200 OK
  - The event was successfully committed to the log.
- 400 BAD_REQUEST
  - There was an issue parsing the request - likely a formatting error.
- 409 CONFLICT
  - The specified version has already been written.