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
	"github.com/ringoid/commons"
	"strings"
	"../apimodel"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"strconv"
)

var anlogger *commons.Logger
var awsDbClient *dynamodb.DynamoDB
var awsDeliveryStreamClient *firehose.Firehose
var awsKinesisClient *kinesis.Kinesis

var deliveryStreamName string
var userProfileTable string
var secretWord string
var commonStreamName string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : get_profile.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : get_profile.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : get_profile.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : get_profile.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "get-profile-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization : get_profile.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_profile.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : get_profile.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_profile.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	commonStreamName, ok = os.LookupEnv("COMMON_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : get_profile.go : env can not be empty COMMON_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_profile.go : start with DELIVERY_STREAM = [%s]", commonStreamName)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : get_profile.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_profile.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : get_profile.go : dynamodb client was successfully initialized")

	awsKinesisClient = kinesis.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : get_profile.go : kinesis client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : get_profile.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_profile.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : get_profile.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.ALBTargetGroupRequest) (events.ALBTargetGroupResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	userAgent := request.Headers["user-agent"]
	if strings.HasPrefix(userAgent, "ELB-HealthChecker") {
		return commons.NewServiceResponse("{}"), nil
	}

	if request.HTTPMethod != "GET" {
		return commons.NewWrongHttpMethodServiceResponse(), nil
	}
	sourceIp := request.Headers["x-forwarded-for"]

	anlogger.Debugf(lc, "get_profile.go : start handle request %v", request)

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "get_profile.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	accessToken, ok := request.QueryStringParameters["accessToken"]
	if !ok {
		errStr = commons.WrongRequestParamsClientError
		anlogger.Errorf(lc, "get_profile.go : accessToken is nil or empty")
		anlogger.Errorf(lc, "get_profile.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	userId, _, _, ok, errStr := commons.Login(appVersion, isItAndroid, accessToken, secretWord, userProfileTable, commonStreamName, awsDbClient, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "get_profile.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	anlogger.Debugf(lc, "get_profile.go : debug print %v %v %v %v", sourceIp, appVersion, isItAndroid, userId)

	resp, ok, errStr := getUserProfile(userId, userProfileTable, lc)
	if !ok {
		anlogger.Errorf(lc, "get_profile.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "get_profile.go : error while marshaling resp object : %v", err)
		return commons.NewServiceResponse(commons.InternalServerError), nil
	}
	anlogger.Debugf(lc, "get_profile.go : return body=%s", string(body))

	return commons.NewServiceResponse(string(body)), nil
}

//return profile, ok and error string
func getUserProfile(userId, userProfileTableName string, lc *lambdacontext.LambdaContext) (*apimodel.GetProfileResponse, bool, string) {
	anlogger.Debugf(lc, "get_profile.go : start fetch user profile for userId [%s]", userId)

	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			commons.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		TableName:      aws.String(userProfileTableName),
		ConsistentRead: aws.Bool(true),
	}

	result, err := awsDbClient.GetItem(input)
	if err != nil {
		anlogger.Errorf(lc, "get_profile.go : error get user profile for userId [%s] : %v", userId, err)
		return nil, false, commons.InternalServerError
	}

	if len(result.Item) == 0 {
		anlogger.Errorf(lc, "get_profile.go : there is no user profile for userId [%s]", userId)
		return nil, false, commons.InternalServerError
	}

	profile := apimodel.GetProfileResponse{}

	customerId, ok, errStr := getStringValueProfileProperty(userId, commons.CustomerIdColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.CustomerId = customerId

	yearOfBirth, ok, errStr := getIntValueProfileProperty(userId, commons.YearOfBirthColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.YearOfBirth = yearOfBirth

	sex, ok, errStr := getStringValueProfileProperty(userId, commons.SexColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.Sex = sex

	profile.LastOnlineText = "Online"
	profile.LastOnlineFlag = "online"
	profile.DistanceText = "unknown"

	property, ok, errStr := getIntValueProfileProperty(userId, commons.UserProfilePropertyColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.Property = property

	transport, ok, errStr := getIntValueProfileProperty(userId, commons.UserProfileTransportColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.Transport = transport

	income, ok, errStr := getIntValueProfileProperty(userId, commons.UserProfileIncomeColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.Income = income

	height, ok, errStr := getIntValueProfileProperty(userId, commons.UserProfileHeightColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.Height = height

	educationLevel, ok, errStr := getIntValueProfileProperty(userId, commons.UserProfileEducationLevelColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.EducationLevel = educationLevel

	hairColor, ok, errStr := getIntValueProfileProperty(userId, commons.UserProfileHairColorColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.HairColor = hairColor

	children, ok, errStr := getIntValueProfileProperty(userId, commons.UserProfileChildrenColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.Children = children

	name, ok, errStr := getStringValueProfileProperty(userId, commons.UserProfileNameColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.Name = name

	jobTitle, ok, errStr := getStringValueProfileProperty(userId, commons.UserProfileJobTitleColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.JobTitle = jobTitle

	company, ok, errStr := getStringValueProfileProperty(userId, commons.UserProfileCompanyColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.Company = company

	eduText, ok, errStr := getStringValueProfileProperty(userId, commons.UserProfileEducationTextColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.EducationText = eduText

	about, ok, errStr := getStringValueProfileProperty(userId, commons.UserProfileAboutColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.About = about

	instagram, ok, errStr := getStringValueProfileProperty(userId, commons.UserProfileInstagramColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.Instagram = instagram

	tiktok, ok, errStr := getStringValueProfileProperty(userId, commons.UserProfileTikTokColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.TikTok = tiktok

	wIlive, ok, errStr := getStringValueProfileProperty(userId, commons.UserProfileWhereILiveColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.WhereLive = wIlive

	wIFrom, ok, errStr := getStringValueProfileProperty(userId, commons.UserProfileWhereIFromColumnName, result, lc)
	if !ok {
		return nil, false, errStr
	}
	profile.WhereFrom = wIFrom

	anlogger.Debugf(lc, "get_profile.go : successfully get user profile [%v] for userId [%s]", profile, userId)

	anlogger.Infof(lc, "get_profile.go : successfully get user profile for userId [%s]", userId)
	return &profile, true, ""
}

//return int value, ok and error string
func getIntValueProfileProperty(userId, propertyName string, result *dynamodb.GetItemOutput, lc *lambdacontext.LambdaContext) (int, bool, string) {
	profilePropertyP, ok := result.Item[propertyName]
	if ok {
		if profilePropertyP.N != nil {
			intV, err := strconv.Atoi(*profilePropertyP.N)
			if err != nil {
				anlogger.Errorf(lc, "get_profile.go : can not convert [%s] to int property (name is [%s]) for userId [%s]",
					*profilePropertyP.N, propertyName, userId)
				return -1, false, commons.InternalServerError
			}
			return intV, true, ""
		}
	}
	return 0, true, ""
}

//return string value, ok and error string
func getStringValueProfileProperty(userId, propertyName string, result *dynamodb.GetItemOutput, lc *lambdacontext.LambdaContext) (string, bool, string) {
	profilePropertyP, ok := result.Item[propertyName]
	if ok {
		if profilePropertyP.S != nil {
			return *profilePropertyP.S, true, ""
		}
	}
	return "unknown", true, ""
}

func main() {
	basicLambda.Start(handler)
}
