package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"../apimodel"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/firehose"
	"os"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/ringoid/commons"
	"strings"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

var anlogger *commons.Logger
var secretWord string
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string
var commonStreamName string
var awsKinesisClient *kinesis.Kinesis

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : claim.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : claim.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : claim.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : claim.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "claim-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization : claim.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : claim.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : claim.go : env can not be empty USER_PROFILE_TABLE")
	}
	anlogger.Debugf(nil, "lambda-initialization : claim.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : claim.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : claim.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : claim.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : claim.go : env can not be empty DELIVERY_STREAM")
	}
	anlogger.Debugf(nil, "lambda-initialization : claim.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : claim.go : firehose client was successfully initialized")

	commonStreamName, ok = os.LookupEnv("COMMON_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : claim.go : env can not be empty COMMON_STREAM")
	}
	anlogger.Debugf(nil, "lambda-initialization : claim.go : start with COMMON_STREAM = [%s]", commonStreamName)

	awsKinesisClient = kinesis.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : claim.go : kinesis client was successfully initialized")
}

func handler(ctx context.Context, request events.ALBTargetGroupRequest) (events.ALBTargetGroupResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	userAgent := request.Headers["user-agent"]
	if strings.HasPrefix(userAgent, "ELB-HealthChecker") {
		return commons.NewServiceResponse("{}"), nil
	}

	if request.HTTPMethod != "POST" {
		return commons.NewWrongHttpMethodServiceResponse(), nil
	}
	sourceIp := request.Headers["x-forwarded-for"]

	anlogger.Debugf(lc, "claim.go : handle request %v", request)

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "claim.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	reqParam, ok := parseParams(request.Body, lc)
	if !ok {
		errStr := commons.WrongRequestParamsClientError
		anlogger.Errorf(lc, "claim.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	userId, _, _, ok, errStr := commons.Login(appVersion, isItAndroid, reqParam.AccessToken, secretWord, userProfileTable, commonStreamName, awsDbClient, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "claim.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	ok, errStr = claim(userId, reqParam.ReferralId, lc)
	if !ok && len(errStr) != 0 {
		anlogger.Errorf(lc, "claim.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	if ok {
		event := commons.NewUserClaimReferralCodeEvent(userId, sourceIp, reqParam.ReferralId)
		commons.SendAnalyticEvent(event, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

		//send common events for neo4j
		partitionKey := userId
		ok, errStr = commons.SendCommonEvent(event, userId, commonStreamName, partitionKey, awsKinesisClient, anlogger, lc)
		if !ok {
			anlogger.Errorf(lc, "claim.go : userId [%s], return %s to client", userId, errStr)
			return commons.NewServiceResponse(errStr), nil
		}
		anlogger.Infof(lc, "claim.go : successfully claim code [%s] for userId [%s]", reqParam.ReferralId, userId)
	}

	resp := commons.BaseResponse{}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "claim.go : error while marshaling resp object %v for userId [%s] : %v", resp, userId, err)
		anlogger.Errorf(lc, "claim.go : userId [%s], return %s to client", userId, errStr)
		return commons.NewServiceResponse(commons.InternalServerError), nil
	}
	anlogger.Debugf(lc, "claim.go : return body=%s to client, userId [%s]", string(body), userId)

	return commons.NewServiceResponse(string(body)), nil
}

//return ok and error string
func claim(userId, code string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "claim.go : claim code [%s] for userId [%s]", code, userId)
	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#referralId": aws.String(commons.ReferralIdColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":referralIdV": {
				S: aws.String(code),
			},
			":referralEmptyIdV": {
				S: aws.String("n/a"),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			commons.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		ConditionExpression: aws.String(fmt.Sprintf("attribute_not_exists(%s) OR #referralId = :referralEmptyIdV", commons.ReferralIdColumnName)),
		TableName:           aws.String(userProfileTable),
		UpdateExpression:    aws.String("SET #referralId = :referralIdV"),
	}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				anlogger.Warnf(lc, "create.go : warning, try to claim with existing referral for userId [%s]", userId)
				return false, ""
			default:
				anlogger.Errorf(lc, "claim.go : error claim code [%s] for userId [%s] : %v", code, userId, aerr)
				return false, commons.InternalServerError
			}
		}
		anlogger.Errorf(lc, "claim.go : error claim code [%s] for userId [%s] : %v", code, userId, err)
		return false, commons.InternalServerError
	}

	anlogger.Debugf(lc, "claim.go : successfully claim code [%s] for userId [%s]", code, userId)
	return true, ""
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.ClaimRequest, bool) {
	var req apimodel.ClaimRequest
	err := json.Unmarshal([]byte(params), &req)

	if err != nil {
		anlogger.Errorf(lc, "claim.go : error unmarshal required params from the string %s : %v", params, err)
		return nil, false
	}

	if req.AccessToken == "" {
		anlogger.Errorf(lc, "claim.go : one of the required param is nil or empty, req %v", req)
		return nil, false
	}

	referealCode := req.ReferralId
	referealCode = strings.TrimSpace(referealCode)
	referealCode = strings.ToLower(referealCode)

	if referealCode == "" {
		anlogger.Errorf(lc, "claim.go : referral code is empty or non exist, code [%s]", referealCode)
		return nil, false
	} else if len([]rune(referealCode)) > apimodel.MaxReferralCodeLength {
		anlogger.Errorf(lc, "claim.go : too big referral code [%s]", referealCode)
		return nil, false
	}

	req.ReferralId = referealCode
	return &req, true
}

func main() {
	basicLambda.Start(handler)
}
