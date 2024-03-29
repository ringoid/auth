package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
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
	"../apimodel"
)

var anlogger *commons.Logger
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string
var secretWord string
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
		fmt.Printf("lambda-initialization : update_profile.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : update_profile.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : update_profile.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : update_profile.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "update-profile-auth"), apimodel.IsDebugLogEnabled)
	if err != nil {
		fmt.Errorf("lambda-initialization : update_profile.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_profile.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : update_profile.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_profile.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : update_profile.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_profile.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : update_profile.go : dynamodb client was successfully initialized")

	commonStreamName, ok = os.LookupEnv("COMMON_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : update_profile.go : env can not be empty COMMON_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_profile.go : start with DELIVERY_STREAM = [%s]", commonStreamName)

	awsKinesisClient = kinesis.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : update_profile.go : kinesis client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : update_profile.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_profile.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : update_profile.go : firehose client was successfully initialized")
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

	anlogger.Debugf(lc, "update_profile.go : start handle request %v", request)

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "update_profile.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	reqParam, ok, errStr := parseParams(request.Body, lc)
	if !ok {
		anlogger.Errorf(lc, "update_profile.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	userId, _, _, ok, errStr := commons.Login(appVersion, isItAndroid, reqParam.AccessToken, secretWord, userProfileTable, commonStreamName, awsDbClient, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "update_profile.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	ok, errStr = updateUserProfile(userId, userProfileTable, reqParam, lc)
	if !ok {
		anlogger.Errorf(lc, "update_profile.go : userId [%s], return %s to client", userId, errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	event := commons.NewUserProfileUpdatedEvent(userId, sourceIp, reqParam.Property, reqParam.Transport, reqParam.Income,
		reqParam.Height, reqParam.Education, reqParam.HairColor, reqParam.Children,
		reqParam.Name, reqParam.JobTitle, reqParam.Company, reqParam.EducationText, reqParam.About, reqParam.Instagram,
		reqParam.TikTok, reqParam.WhereLive, reqParam.WhereFrom, reqParam.StatusText)
	commons.SendAnalyticEvent(event, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	partitionKey := userId
	ok, errStr = commons.SendCommonEvent(event, userId, commonStreamName, partitionKey, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "update_profile.go : userId [%s], return %s to client", userId, errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	resp := commons.BaseResponse{}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "update_profile.go : error while marshaling resp object for userId [%s] : %v", userId, err)
		return commons.NewServiceResponse(commons.InternalServerError), nil
	}
	anlogger.Debugf(lc, "update_profile.go : return body=%s for userId [%s]", string(body), userId)
	//return OK with AccessToken
	return commons.NewServiceResponse(string(body)), nil
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.UpdateProfileRequest, bool, string) {
	anlogger.Debugf(lc, "update_profile.go : parse request body [%s]", params)
	var req apimodel.UpdateProfileRequest
	err := json.Unmarshal([]byte(params), &req)
	if err != nil {
		anlogger.Errorf(lc, "update_profile.go : error marshaling required params from the string [%s] : %v", params, err)
		return nil, false, commons.InternalServerError
	}

	if req.AccessToken == "" {
		anlogger.Errorf(lc, "update_profile.go : empty or nil accessToken request param, req %v", req)
		return nil, false, commons.WrongRequestParamsClientError
	}

	if len(req.Name) == 0 {
		req.Name = "unknown"
	}

	if len(req.JobTitle) == 0 {
		req.JobTitle = "unknown"
	}

	if len(req.Company) == 0 {
		req.Company = "unknown"
	}

	if len(req.EducationText) == 0 {
		req.EducationText = "unknown"
	}

	if len(req.About) == 0 {
		req.About = "unknown"
	}

	if len(req.Instagram) == 0 {
		req.Instagram = "unknown"
	}

	if len(req.TikTok) == 0 {
		req.TikTok = "unknown"
	}

	if len(req.WhereLive) == 0 {
		req.WhereLive = "unknown"
	}

	if len(req.WhereFrom) == 0 {
		req.WhereFrom = "unknown"
	}

	if len(req.StatusText) == 0 {
		req.StatusText = "unknown"
	}

	anlogger.Debugf(lc, "update_profile.go : successfully parse request string [%s] to %v", params, req)
	return &req, true, ""
}

//return ok and error string
func updateUserProfile(userId, userProfileTableName string, req *apimodel.UpdateProfileRequest, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "update_profile.go : start update user profile for userId [%s], profile=%v", userId, req)
	expressionAttrNames := map[string]*string{
		"#property":  aws.String(commons.UserProfilePropertyColumnName),
		"#transport": aws.String(commons.UserProfileTransportColumnName),
		"#income":    aws.String(commons.UserProfileIncomeColumnName),
		"#height":    aws.String(commons.UserProfileHeightColumnName),
		"#edu":       aws.String(commons.UserProfileEducationLevelColumnName),
		"#hair":      aws.String(commons.UserProfileHairColorColumnName),
		"#children":  aws.String(commons.UserProfileChildrenColumnName),
	}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{
		":propertyV":  {N: aws.String(fmt.Sprintf("%v", req.Property))},
		":transportV": {N: aws.String(fmt.Sprintf("%v", req.Transport))},
		":incomeV":    {N: aws.String(fmt.Sprintf("%v", req.Income))},
		":heightV":    {N: aws.String(fmt.Sprintf("%v", req.Height))},
		":eduV":       {N: aws.String(fmt.Sprintf("%v", req.Education))},
		":hairV":      {N: aws.String(fmt.Sprintf("%v", req.HairColor))},
		":childrenV":  {N: aws.String(fmt.Sprintf("%v", req.Children))},
	}
	updateExp := "SET #property = :propertyV, #transport = :transportV, #income = :incomeV, #height = :heightV, #edu = :eduV, #hair = :hairV, #children = :childrenV"

	if len(req.Name) != 0 {
		expressionAttrNames["#name"] = aws.String(commons.UserProfileNameColumnName)
		expressionAttributeValues[":nameV"] = &dynamodb.AttributeValue{
			S: aws.String(req.Name),
		}
		updateExp += ", #name = :nameV"
	}

	if len(req.JobTitle) != 0 {
		expressionAttrNames["#jobTitle"] = aws.String(commons.UserProfileJobTitleColumnName)
		expressionAttributeValues[":jobTitleV"] = &dynamodb.AttributeValue{
			S: aws.String(req.JobTitle),
		}
		updateExp += ", #jobTitle = :jobTitleV"
	}

	if len(req.Company) != 0 {
		expressionAttrNames["#company"] = aws.String(commons.UserProfileCompanyColumnName)
		expressionAttributeValues[":companyV"] = &dynamodb.AttributeValue{
			S: aws.String(req.Company),
		}
		updateExp += ", #company = :companyV"
	}

	if len(req.EducationText) != 0 {
		expressionAttrNames["#educationText"] = aws.String(commons.UserProfileEducationTextColumnName)
		expressionAttributeValues[":educationTextV"] = &dynamodb.AttributeValue{
			S: aws.String(req.EducationText),
		}
		updateExp += ", #educationText = :educationTextV"
	}

	if len(req.About) != 0 {
		expressionAttrNames["#about"] = aws.String(commons.UserProfileAboutColumnName)
		expressionAttributeValues[":aboutV"] = &dynamodb.AttributeValue{
			S: aws.String(req.About),
		}
		updateExp += ", #about = :aboutV"
	}

	if len(req.Instagram) != 0 {
		expressionAttrNames["#instagram"] = aws.String(commons.UserProfileInstagramColumnName)
		expressionAttributeValues[":instagramV"] = &dynamodb.AttributeValue{
			S: aws.String(req.Instagram),
		}
		updateExp += ", #instagram = :instagramV"
	}

	if len(req.TikTok) != 0 {
		expressionAttrNames["#tiktok"] = aws.String(commons.UserProfileTikTokColumnName)
		expressionAttributeValues[":tiktokV"] = &dynamodb.AttributeValue{
			S: aws.String(req.TikTok),
		}
		updateExp += ", #tiktok = :tiktokV"
	}

	if len(req.WhereLive) != 0 {
		expressionAttrNames["#wherelive"] = aws.String(commons.UserProfileWhereILiveColumnName)
		expressionAttributeValues[":whereliveV"] = &dynamodb.AttributeValue{
			S: aws.String(req.WhereLive),
		}
		updateExp += ", #wherelive = :whereliveV"
	}

	if len(req.WhereFrom) != 0 {
		expressionAttrNames["#whereFrom"] = aws.String(commons.UserProfileWhereIFromColumnName)
		expressionAttributeValues[":whereFromV"] = &dynamodb.AttributeValue{
			S: aws.String(req.WhereFrom),
		}
		updateExp += ", #whereFrom = :whereFromV"
	}

	if len(req.StatusText) != 0 {
		expressionAttrNames["#statusText"] = aws.String(commons.UserProfileStatusTextColumnName)
		expressionAttributeValues[":statusTextV"] = &dynamodb.AttributeValue{
			S: aws.String(req.StatusText),
		}
		updateExp += ", #statusText = :statusTextV"
	}

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames:  expressionAttrNames,
			ExpressionAttributeValues: expressionAttributeValues,
			Key: map[string]*dynamodb.AttributeValue{
				commons.UserIdColumnName: {
					S: aws.String(userId),
				},
			},
			TableName:        aws.String(userProfileTableName),
			UpdateExpression: aws.String(updateExp),
		}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "update_profile.go : error update user profile for userId [%s], profile=%v : %v", userId, req, err)
		return false, commons.InternalServerError
	}

	anlogger.Infof(lc, "update_profile.go : successfully update user profile for userId [%s], settings=%v", userId, req)
	return true, ""
}

func main() {
	basicLambda.Start(handler)
}
