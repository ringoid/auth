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
	CustomerId                 string `json:"customerId"`
}

func (event UserAcceptTermsEvent) String() string {
	return fmt.Sprintf("%#v", event)
}

func NewUserAcceptTermsEvent(req *StartReq, sourceIp, userId, customerId string) *UserAcceptTermsEvent {
	return &UserAcceptTermsEvent{
		UserId: userId,
		Locale: req.Locale,
		//gdpr?
		SourceIp:   sourceIp,
		CustomerId: customerId,

		ClientValidationFail:       req.ClientValidationFail,
		UnixTime:                   time.Now().Unix(),
		DateTimeLegalAge:           req.DateTimeLegalAge,
		DateTimePrivacyNotes:       req.DateTimePrivacyNotes,
		DateTimeTermsAndConditions: req.DateTimeTermsAndConditions,

		EventType: "AUTH_USER_ACCEPT_TERMS",
	}
}

type UserVerificationStart struct {
	UserId         string `json:"userId"`
	CountryCode    int    `json:"countryCode"`
	VerifyProvider string `json:"verifyProvider"`
	Locale         string `json:"locale"`
	UnixTime       int64  `json:"unixTime"`
	EventType      string `json:"eventType"`
}

func (event UserVerificationStart) String() string {
	return fmt.Sprintf("%#v", event)
}

func NewUserVerificationStart(userId, provider, locale string, country int) *UserVerificationStart {
	return &UserVerificationStart{
		UserId:         userId,
		VerifyProvider: provider,
		CountryCode:    country,
		Locale:         locale,
		UnixTime:       time.Now().Unix(),
		EventType:      "AUTH_USER_START_VERIFICATION",
	}
}

type UserVerificationCompleteEvent struct {
	UserId              string `json:"userId"`
	CountryCode         int    `json:"countryCode"`
	VerifyProvider      string `json:"verifyProvider"`
	VerificationStartAt int64  `json:"verificationStartAt"`
	Locale              string `json:"locale"`
	UnixTime            int64  `json:"unixTime"`
	EventType           string `json:"eventType"`
}

func (event UserVerificationCompleteEvent) String() string {
	return fmt.Sprintf("%#v", event)
}

func NewUserVerificationCompleteEvent(userId, provider, locale string, country int, startAt int64) *UserVerificationCompleteEvent {
	return &UserVerificationCompleteEvent{
		UserId:              userId,
		CountryCode:         country,
		VerifyProvider:      provider,
		VerificationStartAt: startAt,
		Locale:              locale,
		UnixTime:            time.Now().Unix(),
		EventType:           "AUTH_USER_COMPLETE_VERIFICATION",
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
	return fmt.Sprintf("%#v", event)
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
	SafeDistanceInMeter int    `json:"safeDistanceInMeter"` // 0 (default for men) || 10 (default for women)
	PushMessages        bool   `json:"pushMessages"`        // true (default for men) || false (default for women)
	PushMatches         bool   `json:"pushMatches"`         // true (default)
	PushLikes           string `json:"pushLikes"`           //EVERY (default for men) || 10_NEW (default for women) || 100_NEW
	UnixTime            int64  `json:"unixTime"`
	EventType           string `json:"eventType"`
}

func (event UserSettingsUpdatedEvent) String() string {
	return fmt.Sprintf("%#v", event)
}

func NewUserSettingsUpdatedEvent(settings *UserSettings) *UserSettingsUpdatedEvent {
	return &UserSettingsUpdatedEvent{
		UserId:              settings.UserId,
		SafeDistanceInMeter: settings.SafeDistanceInMeter,
		PushMessages:        settings.PushMessages,
		PushMatches:         settings.PushMatches,
		PushLikes:           settings.PushLikes,
		UnixTime:            time.Now().Unix(),
		EventType:           "AUTH_USER_SETTINGS_UPDATED",
	}
}

type UserLogoutEvent struct {
	UserId    string `json:"userId"`
	UnixTime  int64  `json:"unixTime"`
	EventType string `json:"eventType"`
}

func (event UserLogoutEvent) String() string {
	return fmt.Sprintf("%#v", event)
}

func NewUserLogoutEvent(userId string) *UserLogoutEvent {
	return &UserLogoutEvent{
		UserId:    userId,
		UnixTime:  time.Now().Unix(),
		EventType: "AUTH_USER_LOGOUT",
	}
}

//it's not analytics event
type UserOnlineEvent struct {
	UserId    string `json:"userId"`
	UnixTime  int64  `json:"unixTime"`
	EventType string `json:"eventType"`
}

func (event UserOnlineEvent) String() string {
	return fmt.Sprintf("%#v", event)
}

func NewUserOnlineEvent(userId string) *UserOnlineEvent {
	return &UserOnlineEvent{
		UserId:    userId,
		UnixTime:  time.Now().Unix(),
		EventType: "AUTH_USER_ONLINE",
	}
}
