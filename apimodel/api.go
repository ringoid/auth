package apimodel

import (
	"fmt"
)

type WarmUpRequest struct {
	WarmUpRequest bool `json:"warmUpRequest"`
}

func (req WarmUpRequest) String() string {
	return fmt.Sprintf("[WarmUpRequest={warmUpRequest=%s}]", req.WarmUpRequest)
}

//Request - Response model
type AuthResp struct {
	BaseResponse
	SessionId  string `json:"sessionId"`
	CustomerId string `json:"customerId"`
}

func (resp AuthResp) String() string {
	return fmt.Sprintf("[%v, AuthResp={sessionId=%s, customerId=%s}]", resp.BaseResponse, resp.SessionId, resp.CustomerId)
}

type StartReq struct {
	WarmUpRequest              bool   `json:"warmUpRequest"`
	CountryCallingCode         int    `json:"countryCallingCode"`
	Phone                      string `json:"phone"`
	ClientValidationFail       bool   `json:"clientValidationFail"`
	Locale                     string `json:"locale"`
	DateTimeTermsAndConditions int64  `json:"dtTC"`
	DateTimePrivacyNotes       int64  `json:"dtPN"`
	DateTimeLegalAge           int64  `json:"dtLA"`
}

func (req StartReq) String() string {
	return fmt.Sprintf("[StartReq={countryCallingCode=%s, phone=%s, clientValidationFail=%s, locale=%s, dtTC=%v, dtPN=%v, dtLA=%v}]",
		req.CountryCallingCode, req.Phone, req.ClientValidationFail, req.Locale, req.DateTimeTermsAndConditions, req.DateTimePrivacyNotes, req.DateTimeLegalAge)
}

type VerifyReq struct {
	WarmUpRequest    bool   `json:"warmUpRequest"`
	SessionId        string `json:"sessionId"`
	VerificationCode string `json:"verificationCode"`
}

func (req VerifyReq) String() string {
	return fmt.Sprintf("[VerifyReq={sessionId=%s, verificationCode=%s}]", req.SessionId, req.VerificationCode)
}

type VerifyResp struct {
	BaseResponse
	AccessToken         string `json:"accessToken"`
	AccountAlreadyExist bool   `json:"accountAlreadyExist"`
}

func (resp VerifyResp) GoString() string {
	return fmt.Sprintf("[%v, VerifyResp={accessToken=%s, accountAlreadyExist=%s}]", resp.BaseResponse, resp.AccessToken, resp.AccountAlreadyExist)
}

type CreateReq struct {
	WarmUpRequest bool   `json:"warmUpRequest"`
	AccessToken   string `json:"accessToken"`
	YearOfBirth   int    `json:"yearOfBirth"`
	Sex           string `json:"sex"`
}

func (req CreateReq) String() string {
	return fmt.Sprintf("[CreateReq={accessToken=%s, yearOfBirth=%s, sex=%s}]", req.AccessToken, req.YearOfBirth, req.Sex)
}

type InternalGetUserIdReq struct {
	WarmUpRequest bool   `json:"warmUpRequest"`
	AccessToken string `json:"accessToken"`
}

func (req InternalGetUserIdReq) String() string {
	return fmt.Sprintf("[InternalGetUserIdReq={accessToken=%s}]", req.AccessToken)
}

type InternalGetUserIdResp struct {
	BaseResponse
	UserId string `json:"userId"`
}

func (resp InternalGetUserIdResp) String() string {
	return fmt.Sprintf("[%v, InternalGetUserIdResp={userId=%s}]", resp.BaseResponse, resp.UserId)
}

type UpdateSettingsReq struct {
	WarmUpRequest       bool   `json:"warmUpRequest"`
	AccessToken         string `json:"accessToken"`
	WhoCanSeePhoto      string `json:"whoCanSeePhoto"`      //OPPOSITE (default) || INCOGNITO || ONLY_ME
	SafeDistanceInMeter int    `json:"safeDistanceInMeter"` // 0 (default for men) || 10 (default for women)
	PushMessages        bool   `json:"pushMessages"`        // true (default for men) || false (default for women)
	PushMatches         bool   `json:"pushMatches"`         // true (default)
	PushLikes           string `json:"pushLikes"`           //EVERY (default for men) || 10_NEW (default for women) || 100_NEW || NONE
}

func (req UpdateSettingsReq) String() string {
	return fmt.Sprintf("[UpdateSettingsReq={accessToken=%s, whoCanSeePhoto=%s, safeDistanceInMeter=%d, pushMessages=%v, pushMatches=%v, pushLikes=%s}]",
		req.AccessToken, req.WhoCanSeePhoto, req.SafeDistanceInMeter, req.PushMessages, req.PushMatches, req.PushLikes)
}

type GetSettingsResp struct {
	BaseResponse
	WhoCanSeePhoto      string `json:"whoCanSeePhoto"`      //OPPOSITE (default) || INCOGNITO || ONLY_ME
	SafeDistanceInMeter int    `json:"safeDistanceInMeter"` // 0 (default for men) || 10 (default for women)
	PushMessages        bool   `json:"pushMessages"`        // true (default for men) || false (default for women)
	PushMatches         bool   `json:"pushMatches"`         // true (default)
	PushLikes           string `json:"pushLikes"`           //EVERY (default for men) || 10_NEW (default for women) || 100_NEW || NONE
}

func (resp GetSettingsResp) String() string {
	return fmt.Sprintf("[%v, GetSettingsResp={whoCanSeePhoto=%s, safeDistanceInMeter=%d, pushMessages=%v, pushMatches=%v, pushLikes=%s}]",
		resp.BaseResponse, resp.WhoCanSeePhoto, resp.SafeDistanceInMeter, resp.PushMessages, resp.PushMatches, resp.PushLikes)
}

type LogoutReq struct {
	WarmUpRequest bool   `json:"warmUpRequest"`
	AccessToken   string `json:"accessToken"`
}

func (req LogoutReq) String() string {
	return fmt.Sprintf("[LogoutReq={accessToken=%s}]", req.AccessToken)
}
