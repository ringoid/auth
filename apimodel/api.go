package apimodel

import (
	"fmt"
	"github.com/ringoid/commons"
)

type CreateReq struct {
	WarmUpRequest              bool   `json:"warmUpRequest"`
	YearOfBirth                int    `json:"yearOfBirth"`
	Sex                        string `json:"sex"`
	Locale                     string `json:"locale"`
	DateTimeTermsAndConditions int64  `json:"dtTC"`
	DateTimePrivacyNotes       int64  `json:"dtPN"`
	DateTimeLegalAge           int64  `json:"dtLA"`
	DeviceModel                string `json:"deviceModel"`
	OsVersion                  string `json:"osVersion"`
	ReferralId                 string `json:"referralId"`
	PrivateKey                 string `json:"privateKey"`
}

func (req CreateReq) String() string {
	return fmt.Sprintf("%#v", req)
}

type CreateResp struct {
	commons.BaseResponse
	AccessToken string `json:"accessToken"`
	CustomerId  string `json:"customerId"`
}

func (resp CreateResp) String() string {
	return fmt.Sprintf("%#v", resp)
}

type UpdateSettingsReq struct {
	WarmUpRequest       bool   `json:"warmUpRequest"`
	AccessToken         string `json:"accessToken"`
	SafeDistanceInMeter int    `json:"safeDistanceInMeter"` // 0 (default for men) || 10 (default for women)
	PushMessages        bool   `json:"pushMessages"`        // true (default for men) || false (default for women)
	PushMatches         bool   `json:"pushMatches"`         // true (default)
	PushLikes           string `json:"pushLikes"`           //EVERY (default for men) || 10_NEW (default for women) || 100_NEW || NONE
}

func (req UpdateSettingsReq) String() string {
	return fmt.Sprintf("%#v", req)
}

type GetSettingsResp struct {
	commons.BaseResponse
	SafeDistanceInMeter int    `json:"safeDistanceInMeter"` // 0 (default for men) || 25 (default for women)
	PushMessages        bool   `json:"pushMessages"`        // true (default for men) || false (default for women)
	PushMatches         bool   `json:"pushMatches"`         // true (default)
	PushLikes           string `json:"pushLikes"`           //EVERY (default for men) || 10_NEW (default for women) || 100_NEW || NONE
}

func (resp GetSettingsResp) String() string {
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
