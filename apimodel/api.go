package apimodel

import (
	"fmt"
	"github.com/ringoid/commons"
)

const (
	MaxReferralCodeLength = 256
	IsDebugLogEnabled = false
)

type CreateReq struct {
	Email                      string   `json:"email"`
	AuthSessionId              string   `json:"authSessionId"`
	YearOfBirth                int      `json:"yearOfBirth"`
	Sex                        string   `json:"sex"`
	DateTimeTermsAndConditions int64    `json:"dtTC"`
	DateTimePrivacyNotes       int64    `json:"dtPN"`
	DateTimeLegalAge           int64    `json:"dtLA"`
	DeviceModel                string   `json:"deviceModel"`
	OsVersion                  string   `json:"osVersion"`
	ReferralId                 string   `json:"referralId"`
	PrivateKey                 string   `json:"privateKey"`
	AppSettings                Settings `json:"settings"`
}

func (req CreateReq) String() string {
	return fmt.Sprintf("%#v", req)
}

type CreateResp struct {
	commons.BaseResponse
	AccessToken string `json:"accessToken"`
	CustomerId  string `json:"customerId"`
}

type Settings struct {
	Locale         string `json:"locale"`
	Push           bool   `json:"push"`
	PushNewLike    bool   `json:"pushNewLike"`
	PushNewMessage bool   `json:"pushNewMessage"`
	PushNewMatch   bool   `json:"pushNewMatch"`
	TimeZone       int    `json:"timeZone"`
}

func (resp Settings) String() string {
	return fmt.Sprintf("%#v", resp)
}

func (resp CreateResp) String() string {
	return fmt.Sprintf("%#v", resp)
}

type DeleteReq struct {
	WarmUpRequest bool   `json:"warmUpRequest"`
	AccessToken   string `json:"accessToken"`
}

func (req DeleteReq) String() string {
	return fmt.Sprintf("%#v", req)
}

type ClaimRequest struct {
	AccessToken string `json:"accessToken"`
	ReferralId  string `json:"referralId"`
}

func (req ClaimRequest) String() string {
	return fmt.Sprintf("%#v", req)
}

type UpdateProfileRequest struct {
	AccessToken   string `json:"accessToken"`
	Property      int    `json:"property"`
	Transport     int    `json:"transport"`
	Income        int    `json:"income"`
	Height        int    `json:"height"`
	Education     int    `json:"educationLevel"`
	HairColor     int    `json:"hairColor"`
	Children      int    `json:"children"`
	Name          string `json:"name"`
	JobTitle      string `json:"jobTitle"`
	Company       string `json:"company"`
	EducationText string `json:"education"`
	About         string `json:"about"`
	Instagram     string `json:"instagram"`
	TikTok        string `json:"tikTok"`
	WhereLive     string `json:"whereLive"`
	WhereFrom     string `json:"whereFrom"`
	StatusText    string `json:"statusText"`
}

func (req UpdateProfileRequest) String() string {
	return fmt.Sprintf("%#v", req)
}

type LoginWithEmailRequest struct {
	Email  string `json:"email"`
	Locale string `json:"locale"`
}

func (req LoginWithEmailRequest) String() string {
	return fmt.Sprintf("%#v", req)
}

type LoginWithEmailResponse struct {
	commons.BaseResponse
	AuthSessionId string `json:"authSessionId"`
}

func (req LoginWithEmailResponse) String() string {
	return fmt.Sprintf("%#v", req)
}

type VerifyEmailRequest struct {
	AuthSessionId string `json:"authSessionId"`
	Email         string `json:"email"`
	PinCode       string `json:"pinCode"`
}

func (req VerifyEmailRequest) String() string {
	return fmt.Sprintf("%#v", req)
}

type VerifyEmailResponse struct {
	commons.BaseResponse
	AccessToken string `json:"accessToken"`
}

func (req VerifyEmailResponse) String() string {
	return fmt.Sprintf("%#v", req)
}

type ChangeEmailRequest struct {
	AccessToken string `json:"accessToken"`
	NewEmail    string `json:"newEmail"`
}

func (req ChangeEmailRequest) String() string {
	return fmt.Sprintf("%#v", req)
}

type GetProfileResponse struct {
	commons.BaseResponse
	CustomerId     string `json:"customerId"`
	LastOnlineText string `json:"lastOnlineText"`
	LastOnlineFlag string `json:"lastOnlineFlag"`
	DistanceText   string `json:"distanceText"`
	YearOfBirth    int    `json:"yearOfBirth"`
	Sex            string `json:"sex"`
	Property       int    `json:"property"`
	Transport      int    `json:"transport"`
	Income         int    `json:"income"`
	Height         int    `json:"height"`
	EducationLevel int    `json:"educationLevel"`
	HairColor      int    `json:"hairColor"`
	Children       int    `json:"children"`
	Name           string `json:"name"`
	JobTitle       string `json:"jobTitle"`
	Company        string `json:"company"`
	EducationText  string `json:"education"`
	About          string `json:"about"`
	Instagram      string `json:"instagram"`
	TikTok         string `json:"tikTok"`
	WhereLive      string `json:"whereLive"`
	WhereFrom      string `json:"whereFrom"`
	StatusText     string `json:"statusText"`
}

func (req GetProfileResponse) String() string {
	return fmt.Sprintf("%#v", req)
}
