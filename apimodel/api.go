package apimodel

import (
	"fmt"
	"github.com/ringoid/commons"
)

const (
	MaxReferralCodeLength = 16
)

type CreateReq struct {
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
