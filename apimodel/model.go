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
	DeviceModel         string
	OsVersion           string
}

func (model UserInfo) String() string {
	return fmt.Sprintf("%#v", model)
}

type UserSettings struct {
	UserId              string
	SafeDistanceInMeter int    // 0 (default for men) || 10 (default for women)
	PushMessages        bool   // true (default for men) || false (default for women)
	PushMatches         bool   // true (default)
	PushLikes           string //EVERY (default for men) || 10_NEW (default for women) || 100_NEW
}

func (model UserSettings) String() string {
	return fmt.Sprintf("%#v", model)
}

func NewDefaultSettings(userId, sex string) *UserSettings {
	if sex == "female" {
		return &UserSettings{
			UserId:              userId,
			SafeDistanceInMeter: 25,
			PushMessages:        false,
			PushMatches:         true,
			PushLikes:           "10_NEW",
		}
	}
	return &UserSettings{
		UserId:              userId,
		SafeDistanceInMeter: 0,
		PushMessages:        true,
		PushMatches:         true,
		PushLikes:           "EVERY",
	}
}

func NewUserSettings(userId string, req *UpdateSettingsReq) *UserSettings {
	return &UserSettings{
		UserId:              userId,
		SafeDistanceInMeter: req.SafeDistanceInMeter,
		PushMessages:        req.PushMessages,
		PushMatches:         req.PushMatches,
		PushLikes:           req.PushLikes,
	}
}
