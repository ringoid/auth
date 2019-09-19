package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"os"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"errors"
	"github.com/ringoid/commons"
	"../apimodel"
)

var anlogger *commons.Logger
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
var userSettingsTable string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : clean.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : clean.go : start with ENV = [%s]\n", env)

	//!!!START : VERY IMPORTANT CODE. NEVER DELETE OR MODIFY!!!
	if env != "test" {
		panic("use clean DB in not safe env")
	}
	//!!!END : VERY IMPORTANT CODE. NEVER DELETE OR MODIFY!!!

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : clean.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : clean.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "internal-clean-db-auth"), apimodel.IsDebugLogEnabled)
	if err != nil {
		fmt.Errorf("lambda-initialization : clean.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : clean.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		fmt.Printf("lambda-initialization : clean.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : clean.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	userSettingsTable, ok = os.LookupEnv("USER_SETTINGS_TABLE")
	if !ok {
		fmt.Printf("lambda-initialization : clean.go : env can not be empty USER_SETTINGS_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : clean.go : start with USER_SETTINGS_TABLE = [%s]", userSettingsTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : clean.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : clean.go : aws session was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : clean.go : dynamodb client was successfully initialized")
}

func handler(ctx context.Context) error {
	lc, _ := lambdacontext.FromContext(ctx)
	err := eraseTable(userProfileTable, commons.UserIdColumnName, lc)
	if err != nil {
		return err
	}
	err = eraseTable(userSettingsTable, commons.PhoneColumnName, lc)
	if err != nil {
		return err
	}
	return nil
}

func eraseTable(tableName, idColumnName string, lc *lambdacontext.LambdaContext) error {
	anlogger.Debugf(lc, "clean.go : start clean [%s] table", tableName)
	var lastEvaluatedKey map[string]*dynamodb.AttributeValue
	for {
		scanInput := &dynamodb.ScanInput{
			ConsistentRead:    aws.Bool(true),
			TableName:         aws.String(tableName),
			ExclusiveStartKey: lastEvaluatedKey,
		}
		scanResult, err := awsDbClient.Scan(scanInput)
		if err != nil {
			return errors.New(fmt.Sprintf("error during scan %s table", tableName))
		}
		items := scanResult.Items
		for _, item := range items {
			id := item[idColumnName].S
			deleteInput := &dynamodb.DeleteItemInput{
				Key: map[string]*dynamodb.AttributeValue{
					idColumnName: {
						S: id,
					},
				},
				TableName: aws.String(tableName),
			}
			_, err = awsDbClient.DeleteItem(deleteInput)
			if err != nil {
				return errors.New(fmt.Sprintf("error during delete from %s table", tableName))
			}
		}
		lastEvaluatedKey = scanResult.LastEvaluatedKey
		if len(lastEvaluatedKey) == 0 {
			break
		}
	}
	anlogger.Debugf(lc, "clean.go : successfully clean [%s] table", tableName)
	return nil
}

func main() {
	basicLambda.Start(handler)
}
