# Event Store

The service provides a simple HTTP API which allows
committing events to a persisted log. This is the command service.
Consuming the event log should be managed through a different query
service (such as [event-reader](https://github.com/tobyjsullivan/event-reader)).

## Getting Started

### AWS Configuration

This service has a dependency on DynamoDB.

#### Create a DynamoDB table as your event store

The DynamoDB table will require a Primary Partition Key called `Entity
ID` (type String) and a Primary Sort Key `Version` (type Number). These
column names are not presently configurable.

Specify the name of your table in the `DYNAMODB_TABLE` environment
variable.

Specify the region of your table in the `AWS_REGION` environment
variable.

#### IAM Credentials

Create IAM credentials for the service with the following permissions:

- DynamoDB Table
  - PutItem
  - Query

Specify the credentials in the `AWS_ACCESS_KEY_ID` and
`AWS_SECRET_ACCESS_KEY` environment variables.

### Running with Docker

1. Build the Docker image
  - `docker build -t event-store .`
2. Create a `.env` file with the following vars
  - `AWS_ACCESS_KEY_ID`
  - `AWS_SECRET_ACCESS_KEY`
  - `AWS_REGION`
  - `DYNAMODB_TABLE`
  - `PORT` (Optional. Default: 3000)
3. Run the docker container
  - `docker run -p 3000:3000 --env-file './.env' event-store`

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