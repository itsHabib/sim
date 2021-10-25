# S.I.M. (Simple Image Manager)

## Testing

### Prereqs
- localstack 
- awslocal
- couchbase
- docker
- go 1.16

To set up the environment:
```bash
docker run --rm -idt -p 4566:4566 -e SERVICES='s3' localstack/localstack:0.12.18

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


IMAGE_STORAGE=sim \
LOCALSTACK_URL='http://localhost:4566' \
COUCHBASE_USERNAME=Administrator \
COUCHBASE_PASSWORD=password \
COUCHBASE_ENDPOINT='localhost:8091' \
COUCHBASE_BUCKET='local' \
go test -tags=integration -v ./internal/images/service
```