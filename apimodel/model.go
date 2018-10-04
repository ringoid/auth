package apimodel

import "fmt"

type UserInfo struct {
	UserId    string
	SessionId string
	//full phone with country code included
	Phone               string
	CountryCode         int
	PhoneNumber         string
	CustomerId          string
	VerifyProvider      string
	VerifyRequestId     string
	VerificationStartAt int64
	Locale              string
}

func (model UserInfo) String() string {
	return fmt.Sprintf("%#v", model)
}

type UserSettings struct {
	UserId              string
	WhoCanSeePhoto      string //OPPOSITE (default) || INCOGNITO || ONLY_ME
	SafeDistanceInMeter int    // 0 (default for men) || 10 (default for women)
	PushMessages        bool   // true (default for men) || false (default for women)
	PushMatches         bool   // true (default)
	PushLikes           string //EVERY (default for men) || 10_NEW (default for women) || 100_NEW
	InAppMessages       bool   //true (default for everybody)
	InAppMatches        bool   //true (default for everybody)
	InAppLikes          string //EVERY (default for everybody) || 10_NEW || 100_NEW || NONE
}

func (model UserSettings) String() string {
	return fmt.Sprintf("%#v", model)
}

func NewDefaultSettings(userId, sex string) *UserSettings {
	if sex == "female" {
		return &UserSettings{
			UserId:              userId,
			WhoCanSeePhoto:      "OPPOSITE",
			SafeDistanceInMeter: 25,
			PushMessages:        false,
			PushMatches:         true,
			PushLikes:           "10_NEW",
			InAppMessages:       true,
			InAppMatches:        true,
			InAppLikes:          "EVERY",
		}
	}
	return &UserSettings{
		UserId:              userId,
		WhoCanSeePhoto:      "OPPOSITE",
		SafeDistanceInMeter: 0,
		PushMessages:        true,
		PushMatches:         true,
		PushLikes:           "EVERY",
		InAppMessages:       true,
		InAppMatches:        true,
		InAppLikes:          "EVERY",
	}
}

func NewUserSettings(userId string, req *UpdateSettingsReq) *UserSettings {
	return &UserSettings{
		UserId:              userId,
		WhoCanSeePhoto:      req.WhoCanSeePhoto,
		SafeDistanceInMeter: req.SafeDistanceInMeter,
		PushMessages:        req.PushMessages,
		PushMatches:         req.PushMatches,
		PushLikes:           req.PushLikes,
		InAppMessages:       true,
		InAppMatches:        true,
		InAppLikes:          "EVERY",
	}
}
