package apimodel

import (
	"time"
	"fmt"
)

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

func (event UserAcceptTermsEvent) String() string {
	return fmt.Sprintf("[UserAcceptTermsEvent={userId=%s, locale=%s, sourceIp=%s, clientValidationFail=%v, unixTime=%v, dtTC=%v, dtPN=%v, dtLA=%v, eventType=%s}]",
		event.UserId, event.Locale, event.SourceIp, event.ClientValidationFail, event.UnixTime, event.DateTimeTermsAndConditions, event.DateTimePrivacyNotes, event.DateTimeLegalAge, event.EventType)
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

func (event UserVerificationCompleteEvent) String() string {
	return fmt.Sprintf("[UserVerificationCompleteEvent={userId=%s, unixTime=%v, eventType=%v}]", event.UserId, event.UnixTime, event.EventType)
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

func (event UserProfileCreatedEvent) String() string {
	return fmt.Sprintf("[UserProfileCreatedEvent={userId=%s, sex=%s, yearOfBirth=%v, unixTime=%v, eventType=%s}]",
		event.UserId, event.Sex, event.YearOfBirth, event.UnixTime, event.EventType)
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
	InAppMessages       bool   `json:"inAppMessages"`       //true (default for everybody)
	InAppMatches        bool   `json:"inAppMatches"`        //true (default for everybody)
	InAppLikes          string `json:"inAppLikes"`          //EVERY (default for everybody) || 10_NEW (default for women) || 100_NEW || NONE
	UnixTime            int64  `json:"unixTime"`
	EventType           string `json:"eventType"`
}

func (event UserSettingsUpdatedEvent) String() string {
	return fmt.Sprintf("[UserSettingsUpdatedEvent={userId=%s, whoCanSeePhoto=%s, safeDistanceInMeter=%d, pushMessages=%v, pushMatches=%v, pushLikes=%v, inAppMessages=%v, inAppMatches=%v, inAppLikes=%v, unixTime=%v, eventType=%s}]",
		event.UserId, event.WhoCanSeePhoto, event.SafeDistanceInMeter, event.PushMessages, event.PushMatches, event.PushLikes, event.InAppMessages, event.InAppMatches, event.InAppLikes, event.UnixTime, event.EventType)
}

func NewUserSettingsUpdatedEvent(settings *UserSettings) *UserSettingsUpdatedEvent {
	return &UserSettingsUpdatedEvent{
		UserId:              settings.UserId,
		WhoCanSeePhoto:      settings.WhoCanSeePhoto,
		SafeDistanceInMeter: settings.SafeDistanceInMeter,
		PushMessages:        settings.PushMessages,
		PushMatches:         settings.PushMatches,
		PushLikes:           settings.PushLikes,
		InAppMessages:       settings.InAppMessages,
		InAppMatches:        settings.InAppMatches,
		InAppLikes:          settings.InAppLikes,
		UnixTime:            time.Now().Unix(),
		EventType:           "AUTH_USER_SETTINGS_UPDATED",
	}
}
