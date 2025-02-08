# NotedOK Web Service

## Build, run and test

Run unit-tests

```
make test
```

Run integration tests (The app needs to be running!)

```
make integration_test
```

Run the project

```
make run
```

Run project with live updates while developing

```
gowatch
```

## Environment Variables

```
NOTEDOK_PORT=:8700
NOTEDOK_ALLOW_ORIGIN=http://127.0.0.1:8080

NOTEDOK_BUCKET=net.artemkv.tests3

NOTEDOK_TLS=false
NOTEDOK_CERT_FILE=cert.pem
NOTEDOK_KEY_FILE=key.unencrypted.pem
```

## API

## Testing

```
rq getfiles -e dev
rq getfiles pageSize=2 -e dev
rq getfiles pageSize=2 continuationToken=1NbUxI1wspHIRjwI... -e dev

-- with existing file: should return
-- with file that does not exist: should give 404
-- using etag: should give 304
rq getfile filename="new file 5.txt" -e dev
rq getfile filename="new file 5.txt" etag="65a8e27d8879283831b664bd8b7f0ad4" -e dev

-- with existing file: should overwrite
-- with file that does not exist: should create new
rq putfile filename="test001.txt" content="test content 001" -e dev

-- with existing file: should give 409
-- with file that does not exist: should create new
rq postfile filename="test002.txt" content="test content 002" -e dev

-- with existing file: should delete
-- with file that does not exist: just return
rq deletefile filename="test002.txt" -e dev

-- with existing source file: should rename
-- with source file that does not exist: should give 404
-- with existing target file: should give 409
-- with target file that does not exist: renames
rq renamefile from="test001.txt" to="test002.txt" -e dev
```
