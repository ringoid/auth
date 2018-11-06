package apimodel

import (
	"fmt"
)

const (
	Region     = "eu-west-1"
	MaxRetries = 3

	TwilioSecretKeyBase = "%s/Twilio/Api/Key"
	TwilioApiKeyName    = "twilio-api-key"

	NexmoSecretKeyBase = "%s/Nexmo/Api/Key"
	NexmoApiKeyName    = "nexmo-api-key"

	NexmoApiSecretKeyBase = "%s/Nexmo/Api/Secret"
	NexmoApiSecretKeyName = "nexmo-api-secret"

	SecretWordKeyName = "secret-word-key"
	SecretWordKeyBase = "%s/SecretWord"

	SessionGSIName = "sessionGSI"

	PhoneColumnName     = "phone"
	UserIdColumnName    = "user_id"
	SessionIdColumnName = "session_id"

	CountryCodeColumnName      = "country_code"
	PhoneNumberColumnName      = "phone_number"
	TokenUpdatedTimeColumnName = "token_updated_at"

	SessionTokenColumnName = "session_token"
	SexColumnName          = "sex"

	YearOfBirthColumnName         = "year_of_birth"
	ProfileCreatedAt              = "profile_created_at"
	CustomerIdColumnName          = "customer_id"
	VerifyProviderColumnName      = "verify_provider"
	VerifyRequestIdColumnName     = "verify_request_id"
	VerificationStatusColumnName  = "verify_status"
	VerificationStartAtColumnName = "verification_start_at"
	LocaleColumnName              = "locale"
	AndroidDeviceModelColumnName  = "android_device"
	AndroidOsVersionColumnName    = "android_os_version"
	IOSDeviceModelColumnName      = "ios_device"
	IOsVersionColumnName          = "ios_version"
	CurrentActiveDeviceIsAndroid  = "current_device_is_android"

	UpdatedTimeColumnName    = "updated_at"
	LastOnlineTimeColumnName = "last_online_time"
	CurrentAndroidBuildNum   = "current_android_buildnum"
	CurrentiOSBuildNum       = "current_ios_buildnum"

	SafeDistanceInMeterColumnName = "safe_distance_in_meter"
	PushMessagesColumnName        = "push_messages"
	PushMatchesColumnName         = "push_matches"
	PushLikesColumnName           = "push_likes"

	AccessTokenUserIdClaim       = "userId"
	AccessTokenSessionTokenClaim = "sessionToken"

	AndroidBuildNum = "x-ringoid-android-buildnum"
	iOSdBuildNum    = "x-ringoid-ios-buildnum"

	InternalServerError           = `{"errorCode":"InternalServerError","errorMessage":"Internal Server Error"}`
	WrongRequestParamsClientError = `{"errorCode":"WrongParamsClientError","errorMessage":"Wrong request params"}`
	PhoneNumberClientError        = `{"errorCode":"PhoneNumberClientError","errorMessage":"Phone number is invalid"}`
	CountryCallingCodeClientError = `{"errorCode":"CountryCallingCodeClientError","errorMessage":"Country code is invalid"}`

	WrongSessionIdClientError        = `{"errorCode":"WrongSessionIdClientError","errorMessage":"Session id is invalid"}`
	NoPendingVerificationClientError = `{"errorCode":"NoPendingVerificationClientError","errorMessage":"No pending verifications found"}`
	WrongVerificationCodeClientError = `{"errorCode":"WrongVerificationCodeClientError","errorMessage":"Wrong verification code"}`

	WrongYearOfBirthClientError   = `{"errorCode":"WrongYearOfBirthClientError","errorMessage":"Wrong year of birth"}`
	WrongSexClientError           = `{"errorCode":"WrongSexClientError","errorMessage":"Wrong sex"}`
	InvalidAccessTokenClientError = `{"errorCode":"InvalidAccessTokenClientError","errorMessage":"Invalid access token"}`

	TooOldAppVersionClientError = `{"errorCode":"TooOldAppVersionClientError","errorMessage":"Too old app version"}`

	Twilio = "Twilio"
	Nexmo  = "Nexmo"
)

type BaseResponse struct {
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

func (resp BaseResponse) String() string {
	return fmt.Sprintf("[BaseResponse={errorCode=%s, errorMessage=%s}", resp.ErrorCode, resp.ErrorMessage)
}

//map contains mapping between country calling code and verification provider
var RoutingRuleMap map[int]string

var MinimalAndroidBuildNum = 70
var MinimaliOSBuildNum = 70

func init() {
	RoutingRuleMap = make(map[int]string)
	RoutingRuleMap[1] = Twilio
	RoutingRuleMap[44] = Twilio

	//used by default
	RoutingRuleMap[-1] = Nexmo
}
