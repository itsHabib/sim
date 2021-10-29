# S.I.M. (Simple Image Manager)
A simple image manager for jpegs, pngs, and gifs using Couchbase and S3.

## Setup
```bash
# local only
docker run --rm -idt -p 4566:4566 -e SERVICES='s3' localstack/localstack:0.12.18

# local only
docker run --rm -idt -p "8091-8094:8091-8094" -p "11210:11210" couchbase/server:7.0.2

awslocal s3api create-bucket --bucket sim

# initialize cluster
couchbase-cli cluster-init \
    --services data \
    --index-storage-setting default \
    --cluster-ramsize 1024 \
    --cluster-index-ramsize 256 \
    --cluster-analytics-ramsize 0 \
    --cluster-eventing-ramsize 0 \
    --cluster-fts-ramsize 0 \
    --cluster-username Administrator \
    --cluster-password password \
    --cluster-name dockercompose

couchbase-cli bucket-create \
    --cluster localhost \
    --username Administrator \
    --password password \
    --bucket 'local' \
    --bucket-type couchbase \
    --bucket-ramsize 512 \
    --wait

couchbase-cli collection-manage \
    --cluster localhost:8091 \
    --username Administrator \
    --password password \
    --bucket 'local' \
    --create-scope default

couchbase-cli collection-manage \
    --cluster localhost:8091 \
    --username Administrator \
    --password password \
    --bucket 'local' \
    --create-collection 'default.images'

cbq -u Administrator -p password -s="CREATE PRIMARY INDEX ON \`local\`.default.images;"
```

## Usage
```bash
# example of required env vars
REGION=us-east-1 
IMAGE_STORAGE=sim 
LOCALSTACK_URL='http://localhost:4566'
COUCHBASE_USERNAME=Administrator
COUCHBASE_PASSWORD=password
COUCHBASE_ENDPOINT='localhost:8091'
COUCHBASE_BUCKET='local'

##  optional env vars

# use if not using real AWS creds
LOCALSTACK_URL='http://localhost:4566'
# use true to enable log messaging
DEBUG=false

# uploads
./sim upload -f /path/to/file.jpg -n file.jpg

# downloads
./sim download -f /path/to/download.jpg --imageId 123

# deletes
./sim deletes --imageId 123

# list
./sim list
```

### Example Demo 
https://share.getcloudapp.com/Z4uryrNg

## Testing

### Running integration tests
```bash
# with local setup

REGION=us-east-1 \
IMAGE_STORAGE=sim \
LOCALSTACK_URL='http://localhost:4566' \
COUCHBASE_USERNAME=Administrator \
COUCHBASE_PASSWORD=password \
COUCHBASE_ENDPOINT='localhost:8091' \
COUCHBASE_BUCKET='local' \
go test -tags=integration -v ./internal/images/service
```