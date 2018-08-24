package apimodel

import "fmt"

//Request - Response model
type AuthResp struct {
	BaseResponse
	SessionId string `json:"sessionId"`
}

func (resp AuthResp) String() string {
	return fmt.Sprintf("[%v, AuthResp={sessionId=%s}]", resp.BaseResponse, resp.SessionId)
}

type StartReq struct {
	CountryCallingCode         int    `json:"countryCallingCode"`
	Phone                      string `json:"phone"`
	ClientValidationFail       bool   `json:"clientValidationFail"`
	Locale                     string `json:"locale"`
	DateTimeTermsAndConditions string `json:"dtTC"`
	DateTimePrivacyNotes       string `json:"dtPN"`
	DateTimeLegalAge           string `json:"dtLA"`
}

func (req StartReq) String() string {
	return fmt.Sprintf("[StartReq={countryCallingCode=%s, phone=%s, clientValidationFail=%s, locale=%s, dtTC=%s, dtPN=%s, dtLA=%s}]",
		req.CountryCallingCode, req.Phone, req.ClientValidationFail, req.Locale, req.DateTimeTermsAndConditions, req.DateTimePrivacyNotes, req.DateTimeLegalAge)
}

type VerifyReq struct {
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
	AccessToken string `json:"accessToken"`
	YearOfBirth int    `json:"yearOfBirth"`
	Sex         string `json:"sex"`
}

func (req CreateReq) String() string {
	return fmt.Sprintf("[CreateReq={accessToken=%s, yearOfBirth=%s, sex=%s}]", req.AccessToken, req.YearOfBirth, req.Sex)
}
