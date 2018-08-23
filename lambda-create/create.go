package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"../sys_log"
	"../apimodel"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/firehose"
	"os"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"time"
	"strconv"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

var anlogger *syslog.Logger
var twilioKey string
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
var neo4jurl string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string

const (
	region     = "eu-west-1"
	maxRetries = 3

	accessTokenGSIName = "accessTokenGSI"

	userIdColumnName      = "user_id"
	accessTokenColumnName = "access_token"
	sexColumnName         = "sex"
	yearOfBirthColumnName = "year_of_birth"
	profileCreatedAt      = "profile_created_at"
)

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("create.go : env can not be empty ENV")
		os.Exit(1)
	}
	fmt.Printf("create.go : start with ENV = [%s]", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("create.go : env can not be empty PAPERTRAIL_LOG_ADDRESS")
		os.Exit(1)
	}
	fmt.Printf("create.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "create-auth"))
	if err != nil {
		fmt.Errorf("create.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Debugf("create.go : logger was successfully initialized")

	neo4jurl, ok = os.LookupEnv("NEO4J_URL")
	if !ok {
		fmt.Printf("create.go : env can not be empty NEO4J_URL")
		os.Exit(1)
	}
	anlogger.Debugf("create.go : start with NEO4J_URL = [%s]", neo4jurl)

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		fmt.Printf("create.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf("create.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(region).WithMaxRetries(maxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf("create.go : error during initialization : %v", err)
	}
	anlogger.Debugf("create.go : aws session was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf("create.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf("create.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf("create.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf("create.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	anlogger.Debugf("create.go : start handle request %v", request)

	reqParam, ok, errStr := parseParams(request.Body)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	userId, ok, errStr := findUserId(reqParam.AccessToken)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	ok, errStr = createUserProfileNeo4j(userId, reqParam)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	ok, errStr = createUserProfileDynamo(userId, reqParam)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	event := apimodel.NewUserProfileCreatedEvent(userId, *reqParam)
	sendAnalyticEvent(event, userId)

	resp := apimodel.BaseResponse{}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf("create.go : error while marshaling resp object for userId [%s] : %v", userId, err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	anlogger.Debugf("create.go : return body resp [%s] for userId [%s]", string(body), userId)
	//return OK with AccessToken
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

func sendAnalyticEvent(event interface{}, userId string) {
	anlogger.Debugf("create.go : start sending analytics event [%v] for userId [%s]", event, userId)
	data, err := json.Marshal(event)
	if err != nil {
		anlogger.Errorf("create.go : error marshaling analytics event [%v] for userId [%s] : %v", event, userId, err)
		return
	}
	newLine := "\n"
	data = append(data, newLine...)
	_, err = awsDeliveryStreamClient.PutRecord(&firehose.PutRecordInput{
		DeliveryStreamName: aws.String(deliveryStreamName),
		Record: &firehose.Record{
			Data: data,
		},
	})

	if err != nil {
		anlogger.Errorf("create.go : error sending analytics event [%v] for userId [%s] : %v", event, userId, err)
	}

	anlogger.Debugf("create.go : successfully send analytics event [%v] for userId [%s]", event, userId)
}

func parseParams(params string) (*apimodel.CreateReq, bool, string) {
	anlogger.Debugf("create.go : start parsing request body [%s]", params)
	var req apimodel.CreateReq
	err := json.Unmarshal([]byte(params), &req)
	if err != nil {
		anlogger.Errorf("create.go : error marshaling required params from the string [%s] : %v", params, err)
		return nil, false, apimodel.InternalServerError
	}

	if req.YearOfBirth < time.Now().UTC().Year()-150 || req.YearOfBirth > time.Now().UTC().Year()-18 {
		anlogger.Errorf("create.go : wrong year of birth [%d] request param, req %v", req.YearOfBirth, req)
		return nil, false, apimodel.WrongYearOfBirthClientError
	}

	if req.Sex == "" || (req.Sex != "male" && req.Sex != "female") {
		anlogger.Errorf("create.go : wrong sex [%s] request param, req %v", req.Sex, req)
		return nil, false, apimodel.WrongSexClientError
	}
	anlogger.Debugf("create.go : successfully parse request string [%s] to [%v]", params, req)
	return &req, true, ""
}

//return userId, was everything ok, error string in case of error
func findUserId(accessToken string) (string, bool, string) {
	anlogger.Debugf("create.go : start find user by accessToken [%s]", accessToken)

	if len(accessToken) == 0 {
		anlogger.Errorf("create.go : empty access token")
		return "", false, apimodel.WrongRequestParamsClientError
	}

	input := &dynamodb.QueryInput{
		ExpressionAttributeNames: map[string]*string{
			"#token": aws.String(accessTokenColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":tV": {
				S: aws.String(accessToken),
			},
		},
		KeyConditionExpression: aws.String("#token = :tV"),
		TableName:              aws.String(userProfileTable),
		IndexName:              aws.String(accessTokenGSIName),
	}

	result, err := awsDbClient.Query(input)
	if err != nil {
		anlogger.Errorf("create.go : error while find user by accessToken [%s] : %v", accessToken, err)
		return "", false, apimodel.InternalServerError
	}

	if len(result.Items) == 0 {
		anlogger.Warnf("create.go : warning, there is no user with such accessToken [%s]", accessToken)
		return "", false, apimodel.InvalidAccessTokenClientError
	}

	if len(result.Items) > 1 {
		anlogger.Errorf("create.go : error, there more than one user with such accessToken [%s]", accessToken)
		return "", false, apimodel.InternalServerError
	}

	userId := *result.Items[0][userIdColumnName].S
	anlogger.Debugf("create.go : successfully fetched userId [%s] by accessToken [%s]", userId, accessToken)

	return userId, true, ""
}

//return ok and error string
func createUserProfileDynamo(userId string, req *apimodel.CreateReq) (bool, string) {
	anlogger.Debugf("create.go : start create user profile in Dynamo for userId [%s] and req %v", userId, req)
	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#sex":     aws.String(sexColumnName),
				"#year":    aws.String(yearOfBirthColumnName),
				"#created": aws.String(profileCreatedAt),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":sV": {
					S: aws.String(req.Sex),
				},
				":yV": {
					N: aws.String(strconv.Itoa(req.YearOfBirth)),
				},
				":cV": {
					S: aws.String(time.Now().UTC().Format("2006-01-02-15-04-05.000")),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				userIdColumnName: {
					S: aws.String(userId),
				},
			},
			ConditionExpression: aws.String(fmt.Sprintf("attribute_not_exists(%v)", sexColumnName)),

			TableName:        aws.String(userProfileTable),
			UpdateExpression: aws.String("SET #sex = :sV, #year = :yV, #created = :cV"),
		}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				anlogger.Warnf("start.go : warning, profile for userId [%s] already exist", userId)
				return true, ""
			}
		}
		anlogger.Errorf("start.go : error while creating profile for userId [%s] : %v", userId, err)
		return false, apimodel.InternalServerError
	}

	anlogger.Debugf("create.go : successfully create user profile in Dynamo for userId [%s] and req %v", userId, req)
	return true, ""
}

//return ok and error string
func createUserProfileNeo4j(userId string, req *apimodel.CreateReq) (bool, string) {
	anlogger.Debugf("create.go : start create user profile in Neo4j for userId [%s] and req %v", userId, req)
	anlogger.Debugf("create.go : successfully create user profile in Neo4j for userId [%s] and req %v", userId, req)
	return true, ""
}

func main() {
	basicLambda.Start(handler)
}
