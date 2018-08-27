package apimodel

import "time"

type UserAcceptTermsEvent struct {
	UserId                     string `json:"userId"`
	Locale                     string `json:"locale"`
	SourceIp                   string `json:"sourceIp"`
	ClientValidationFail       bool   `json:"clientValidationFail"`
	UnixTime                   int64  `json:"unixTime"`
	EventType                  string `json:"eventType"`
	DateTimeTermsAndConditions int64  `json:"dtTC"`
	DateTimePrivacyNotes       int64  `json:"dtPN"`
	DateTimeLegalAge           int64  `json:"dtLA"`
}

func NewUserAcceptTermsEvent(req *StartReq, sourceIp, userId string) *UserAcceptTermsEvent {
	return &UserAcceptTermsEvent{
		UserId: userId,
		Locale: req.Locale,
		//gdpr?
		SourceIp: sourceIp,

		ClientValidationFail:       req.ClientValidationFail,
		UnixTime:                   time.Now().Unix(),
		DateTimeLegalAge:           req.DateTimeLegalAge,
		DateTimePrivacyNotes:       req.DateTimePrivacyNotes,
		DateTimeTermsAndConditions: req.DateTimeTermsAndConditions,

		EventType: "AUTH_USER_ACCEPT_TERMS",
	}
}

type UserVerificationCompleteEvent struct {
	UserId    string `json:"userId"`
	UnixTime  int64  `json:"unixTime"`
	EventType string `json:"eventType"`
}

func NewUserVerificationCompleteEvent(userId string) *UserVerificationCompleteEvent {
	return &UserVerificationCompleteEvent{
		UserId:    userId,
		UnixTime:  time.Now().Unix(),
		EventType: "AUTH_USER_COMPLETE_VERIFICATION",
	}
}

type UserProfileCreatedEvent struct {
	UserId      string `json:"userId"`
	Sex         string `json:"sex"`
	YearOfBirth int    `json:"yearOfBirth"`
	UnixTime    int64  `json:"unixTime"`
	EventType   string `json:"eventType"`
}

func NewUserProfileCreatedEvent(userId string, req *CreateReq) *UserProfileCreatedEvent {
	return &UserProfileCreatedEvent{
		UserId:      userId,
		Sex:         req.Sex,
		YearOfBirth: req.YearOfBirth,
		UnixTime:    time.Now().Unix(),
		EventType:   "AUTH_USER_PROFILE_CREATED",
	}
}

type UserSettingsUpdatedEvent struct {
	UserId              string `json:"userId"`
	WhoCanSeePhoto      string `json:"whoCanSeePhoto"`      //OPPOSITE (default) || INCOGNITO || ONLY_ME
	SafeDistanceInMeter int    `json:"safeDistanceInMeter"` // 0 (default for men) || 10 (default for women)
	PushMessages        bool   `json:"pushMessages"`        // true (default for men) || false (default for women)
	PushMatches         bool   `json:"pushMatches"`         // true (default)
	PushLikes           string `json:"pushLikes"`           //EVERY (default for men) || 10_NEW (default for women) || 100_NEW
	UnixTime            int64  `json:"unixTime"`
	EventType           string `json:"eventType"`
}

func NewUserSettingsUpdatedEvent(settings *UserSettings) *UserSettingsUpdatedEvent {
	return &UserSettingsUpdatedEvent{
		UserId:              settings.UserId,
		WhoCanSeePhoto:      settings.WhoCanSeePhoto,
		SafeDistanceInMeter: settings.SafeDistanceInMeter,
		PushMessages:        settings.PushMessages,
		PushMatches:         settings.PushMatches,
		PushLikes:           settings.PushLikes,
		UnixTime:            time.Now().Unix(),
		EventType:           "AUTH_USER_SETTINGS_UPDATED",
	}
}
