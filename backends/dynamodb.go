//go:build storage_lock_dynamodb

package backends

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	dynamodb_storage "github.com/storage-lock/go-dynamodb-storage"
	"github.com/storage-lock/go-storage"
	framework "github.com/storage-lock/go-storage-lock-test"
)

func init() {
	framework.RegisterBackend("dynamodb", func(ctx context.Context) (storage.Storage, error) {
		endpoint, err := framework.BackendFromEnv("STORAGE_LOCK_DYNAMODB_ENDPOINT")
		if err != nil {
			return nil, err
		}
		// 存储实现只接受现成客户端，端点到客户端的转换放在这里。
		// dynamodb-local 不校验凭据，静态占位值即可。
		client := dynamodb.New(dynamodb.Options{
			Region:       "us-east-1",
			BaseEndpoint: aws.String(endpoint),
			Credentials:  aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider("local", "local", "")),
		})

		// Init 不建表，而条件写直接打表。表不存在时每个用例都会失败，
		// 所以在交给套件之前建好。已存在不算错误。
		table := storage.DefaultStorageTableName
		_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
			TableName: aws.String(table),
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("LockId"), AttributeType: types.ScalarAttributeTypeS},
			},
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("LockId"), KeyType: types.KeyTypeHash},
			},
			BillingMode: types.BillingModePayPerRequest,
		})
		if err != nil {
			var inUse *types.ResourceInUseException
			if !errors.As(err, &inUse) {
				return nil, err
			}
		}

		options := dynamodb_storage.NewDynamoDBStorageOptions().
			SetConnectionManager(dynamodb_storage.NewDynamoDBConnectionManagerFromClient(client)).
			SetTableName(table)
		return dynamodb_storage.NewDynamoDBStorage(ctx, options)
	})
}
