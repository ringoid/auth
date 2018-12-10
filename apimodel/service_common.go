package apimodel

import (
	"time"
	"github.com/ringoid/commons"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/satori/go.uuid"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"fmt"
)

//return ok and error string
func DeleteUserFromAuthService(userId, userProfileTableName, userSettingsTableName string, awsDbClient *dynamodb.DynamoDB,
	anlogger *commons.Logger, lc *lambdacontext.LambdaContext) (bool, string) {

	anlogger.Debugf(lc, "service_common.go : delete user from the service (%s and %s) tables, userId [%s]",
		userProfileTableName, userSettingsTableName, userId)

	if ok, errStr := deleteFromTable(userId, userProfileTableName, awsDbClient, anlogger, lc); !ok {
		return ok, errStr
	}

	if ok, errStr := deleteFromTable(userId, userSettingsTableName, awsDbClient, anlogger, lc); !ok {
		return ok, errStr
	}

	anlogger.Infof(lc, "service_common.go : successfully delete user from the service, userId [%s]", userId)

	return true, ""
}

func deleteFromTable(userId, tableName string, awsDbClient *dynamodb.DynamoDB, anlogger *commons.Logger, lc *lambdacontext.LambdaContext) (bool, string) {
	deleteInput := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			commons.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		TableName: aws.String(tableName),
	}
	_, err := awsDbClient.DeleteItem(deleteInput)
	if err != nil {
		anlogger.Errorf(lc, "service_common.go : error delete user from table [%s], userId [%s]", tableName, userId)
		return false, commons.InternalServerError
	}
	return true, ""
}

//return ok, errorString if not ok
func DisableCurrentAccessToken(userId, tableName string, awsDbClient *dynamodb.DynamoDB, anlogger *commons.Logger, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "service_common.go : disable current access token for userId [%s]", userId)

	newSessionToken, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "service_common.go : error while generate new sessionToken for userId [%s] : %v", userId, err)
		return false, commons.InternalServerError
	}

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#token":     aws.String(commons.SessionTokenColumnName),
			"#updatedAt": aws.String(commons.TokenUpdatedTimeColumnName),
			"#status":    aws.String(commons.UserStatusColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":tV": {
				S: aws.String(newSessionToken.String()),
			},
			":uV": {
				S: aws.String(time.Now().UTC().Format("2006-01-02-15-04-05.000")),
			},
			":statusV": {
				S: aws.String(commons.UserHiddenStatus),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			commons.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		ConditionExpression: aws.String(fmt.Sprintf("attribute_exists(%v)", commons.UserIdColumnName)),
		TableName:           aws.String(tableName),
		UpdateExpression:    aws.String("SET #token = :tV, #updatedAt = :uV, #status = :statusV"),
	}

	_, err = awsDbClient.UpdateItem(input)

	if err != nil {
		anlogger.Errorf(lc, "service_common.go : error disable current access token for userId [%s] : %v", userId, err)
		return false, commons.InternalServerError
	}

	anlogger.Infof(lc, "service_common.go : successfully disable current access token for userId [%s]", userId)
	return true, ""
}
