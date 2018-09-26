package apimodel

import (
	"../sys_log"
	"fmt"
	"net/http"
	"strings"
	"io/ioutil"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"encoding/json"
)

//ok and error string
func StartTwilioVerify(code int, number, locale, secretKey string, anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "twillio.go : verify phone with code [%d] and phone [%s]", code, number)

	params := fmt.Sprintf("via=sms&&phone_number=%s&&country_code=%d", number, code)
	if len(locale) != 0 {
		params = fmt.Sprintf("via=sms&&phone_number=%s&&country_code=%d&&locale=%s", number, code, locale)
	}
	url := "https://api.authy.com/protected/json/phones/verification/start"

	req, err := http.NewRequest("POST", url, strings.NewReader(params))

	if err != nil {
		anlogger.Errorf(lc, "twillio.go : error while construct the request to verify code [%d] and phone [%s] : %v", code, number, err)
		return false, InternalServerError
	}

	req.Header.Set("X-Authy-API-Key", secretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}

	anlogger.Debugf(lc, "twillio.go : make POST request by url %s with params %s", url, params)

	resp, err := client.Do(req)
	if err != nil {
		anlogger.Errorf(lc, "twillio.go error while making request by url %s with params %s : %v", url, params, err)
		return false, InternalServerError
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		anlogger.Errorf(lc, "twillio.go : error reading response body from Twilio, code [%d] and phone [%s] : %v", code, number, err)
		return false, InternalServerError
	}

	anlogger.Debugf(lc, "twillio.go : receive response from Twilio, body=%s, code [%d] and phone [%s]", string(body), code, number)
	if resp.StatusCode != 200 {
		anlogger.Errorf(lc, "twillio.go : error while sending sms, status %v, body %v",
			resp.StatusCode, string(body))

		var errorResp map[string]interface{}
		err := json.Unmarshal(body, &errorResp)
		if err != nil {
			anlogger.Errorf(lc, "twillio.go : error parsing Twilio response, code [%d] and phone [%s] : %v", code, number, err)
			return false, InternalServerError
		}

		if errorCodeObject, ok := errorResp["error_code"]; ok {
			if errorCodeStr, ok := errorCodeObject.(string); ok {
				anlogger.Errorf(lc, "twillio.go : Twilio return error_code=%s, code [%d] and phone [%s]", errorCodeStr, code, number, err)
				switch errorCodeStr {
				case "60033":
					return false, PhoneNumberClientError
				case "60078":
					return false, CountryCallingCodeClientError
				}
			}
		}

		return false, InternalServerError
	}

	anlogger.Infof(lc, "twillio.go : sms was successfully sent, status %v, body %v, code [%d] and phone [%s]",
		resp.StatusCode, string(body), code, number)
	return true, ""
}

//return ok and error string if not
func CompleteTwilioVerify(userInfo *UserInfo, verificationCode, secretKey string, anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "twillio.go : verify phone for userId [%s], userInfo=%v", userInfo.UserId, userInfo)

	url := fmt.Sprintf("https://api.authy.com/protected/json/phones/verification/check?phone_number=%s&country_code=%d&verification_code=%s",
		userInfo.PhoneNumber, userInfo.CountryCode, verificationCode)

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		anlogger.Errorf(lc, "twillio.go : error while construct the request, userId [%s] : %v", userInfo.UserId, err)
		return false, InternalServerError
	}

	req.Header.Set("X-Authy-API-Key", secretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}

	anlogger.Debugf(lc, "twillio.go : make GET request by url %s, userId [%s]", url, userInfo.UserId)

	resp, err := client.Do(req)
	if err != nil {
		anlogger.Fatalf(lc, "twillio.go : error while making GET request, userId [%s] : %v", userInfo.UserId, err)
		return false, InternalServerError
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		anlogger.Errorf(lc, "twillio.go : error reading response body from Twilio, userId [%s] : %v", userInfo.UserId, err)
		return false, InternalServerError
	}
	anlogger.Debugf(lc, "twillio.go : receive response from Twilio, body=%s, userId [%s]", string(body), userInfo.UserId)
	if resp.StatusCode != 200 {
		anlogger.Errorf(lc, "twillio.go : error while sending sms, status %v, body %v, userId [%s]",
			resp.StatusCode, string(body), userInfo.UserId)

		var errorResp map[string]interface{}
		err := json.Unmarshal(body, &errorResp)
		if err != nil {
			anlogger.Errorf(lc, "twillio.go : error parsing Twilio response, body=%s, userId [%s] : %v", body, userInfo.UserId, err)
			return false, InternalServerError
		}

		if errorCodeObject, ok := errorResp["error_code"]; ok {
			if errorCodeStr, ok := errorCodeObject.(string); ok {
				anlogger.Errorf(lc, "twillio.go : error verify phone, error_code=%s, userId [%s]", errorCodeStr, userInfo.UserId)
				switch errorCodeStr {
				case "60023":
					return false, NoPendingVerificationClientError
				case "60022":
					return false, WrongVerificationCodeClientError
				}
			}
		}

		return false, InternalServerError
	}

	anlogger.Debugf(lc, "twillio.go : successfully complete verification for userId [%s], userInfo=%v",
		userInfo.UserId, userInfo)
	return true, ""
}
