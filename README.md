# S.I.M. (Simple Image Manager)

## Testing

### Prereqs
- localstack 
- awslocal
- docker
- go 1.16

There is only one basic test for now that ensures we are writing and reading
from the db properly.

To run the test:
```bash
docker run --rm -it -p 4566:4566 -e SERVICES='dynamodb' localstack/localstack:0.12.18

awslocal dynamodb create-table --table-name sim \
--attribute-definitions AttributeName=id,AttributeType=S \
--key-schema AttributeName=id,KeyType=HASH \
--provisioned-throughput ReadCapacityUnits=10,WriteCapacityUnits=5

TABLE_NAME='sim' LOCALSTACK_URL='http://localhost:4566' go test -tags=integration -v ./internal/images/service
```