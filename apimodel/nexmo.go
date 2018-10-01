package apimodel

import (
	"../sys_log"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"net/http"
	"fmt"
	"encoding/json"
	"bytes"
	"io/ioutil"
)

//request_id, ok and error string
func StartNexmoVerify(code int, number, apiKey, apiSecret, brand, userId string, anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) (string, bool, string) {
	anlogger.Debugf(lc, "nexmo.go : verify phone with code [%d] and phone [%s] with brand [%s] for userId [%s]", code, number, brand, userId)
	url := "https://api.nexmo.com/verify/json"
	bodyMap := make(map[string]interface{})
	bodyMap["api_key"] = apiKey
	bodyMap["api_secret"] = apiSecret
	bodyMap["number"] = fmt.Sprintf("%d%s", code, number)
	bodyMap["brand"] = brand
	bodyMap["pin_expiry"] = 600 //10 mins

	bodyStr, err := json.Marshal(bodyMap)
	if err != nil {
		anlogger.Errorf(lc, "nexmo.go : start verification, error marshaling body %v for userId [%s] : %v", bodyMap, userId, err)
		return "", false, InternalServerError
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyStr))

	if err != nil {
		anlogger.Errorf(lc, "nexmo.go : start verification, error while construct the request to verify code [%d] and phone [%s] for userId [%s] : %v", code, number, userId, err)
		return "", false, InternalServerError
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	anlogger.Debugf(lc, "nexmo.go : start verification, make POST request by url %s with body %s for userId [%s]", url, string(bodyStr), userId)

	resp, err := client.Do(req)
	if err != nil {
		anlogger.Errorf(lc, "nexmo.go start verification, error while making request by url %s with body %s for userId [%s] : %v", url, string(bodyStr), userId, err)
		return "", false, InternalServerError
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		anlogger.Errorf(lc, "nexmo.go : start verification, error reading response body from Nexmo, code [%d] and phone [%s] for userId [%s] : %v", code, number, userId, err)
		return "", false, InternalServerError
	}

	anlogger.Debugf(lc, "nexmo.go : start verification, receive response from Nexmo, body=%s, code [%d] and phone [%s] for userId [%s]", string(body), code, number, userId)

	var nexmoResp map[string]interface{}
	err = json.Unmarshal(body, &nexmoResp)
	if err != nil {
		anlogger.Errorf(lc, "nexmo.go : start verification, error marshaling Nexmo response, code [%d] and phone [%s] for userId [%s] : %v", code, number, userId, err)
		return "", false, InternalServerError
	}

	if status, ok := nexmoResp["status"]; ok {
		if statusStr, ok := status.(string); ok {
			anlogger.Debugf(lc, "nexmo.go : start verification, Nexmo return status [%s] for start verification of code [%d] and phone [%s], response %v for userId [%s]", status, code, number, nexmoResp, userId)
			var errorText string
			switch statusStr {
			case "0":
				requestId := nexmoResp["request_id"].(string)
				anlogger.Debugf(lc, "nexmo.go : start verification request was successfully accepted by Nexmo for code [%d] and phone [%s], request_id [%s] for userId [%s]", code, number, requestId, userId)
				return requestId, true, ""
			case "10":
				anlogger.Warnf(lc, "nexmo.go : start verification, concurrent verifications to the same number are not allowed for code [%d] and phone [%s] for userId [%s]", code, number, userId)
				return "", true, ""
			case "3":
				errorText = nexmoResp["error_text"].(string)
				anlogger.Errorf(lc, "nexmo.go : start verification error, invalid value of parameter, error text [%s] for code [%d] and phone [%s] for userId [%s]", errorText, code, number, userId)
				//todo:make difference between code and numbers error
				//return "", false, CountryCallingCodeClientError
				return "", false, PhoneNumberClientError
			default:
				errorText = nexmoResp["error_text"].(string)
				anlogger.Errorf(lc, "nexmo.go : start verification error, error text [%s] for code [%d] and phone [%s] for userId [%s]", errorText, code, number, userId)
				return "", false, InternalServerError
			}
		}
	}

	anlogger.Errorf(lc, "nexmo.go : start verification error, nexmo response does not contain required field [status], response %v for userId [%s]", nexmoResp, userId)
	return "", false, InternalServerError
}

//return ok and error string
func CompleteNexmoVerify(userInfo *UserInfo, verificationCode, apiKey, apiSecret string, anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "nexmo.go : complete verification phone for userId [%s], verification code [%s], request id [%s], userInfo=%v", userInfo.UserId, verificationCode, userInfo.VerifyRequestId, userInfo)
	url := "https://api.nexmo.com/verify/check/json"

	bodyMap := make(map[string]interface{})
	bodyMap["api_key"] = apiKey
	bodyMap["api_secret"] = apiSecret
	bodyMap["request_id"] = userInfo.VerifyRequestId
	bodyMap["code"] = verificationCode

	bodyStr, err := json.Marshal(bodyMap)
	if err != nil {
		anlogger.Errorf(lc, "nexmo.go : complete verification, error marshaling body %v for userId [%s]: %v", bodyMap, userInfo.UserId, err)
		return false, InternalServerError
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyStr))

	if err != nil {
		anlogger.Errorf(lc, "nexmo.go : complete verification, error while construct the request to verify request id [%s] with code [%s] for userId [%s] : %v", userInfo.VerifyRequestId, verificationCode, userInfo.UserId, err)
		return false, InternalServerError
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	anlogger.Debugf(lc, "nexmo.go : complete verification, make POST request by url %s with body %s for userId [%s]", url, string(bodyStr), userInfo.UserId)

	resp, err := client.Do(req)
	if err != nil {
		anlogger.Errorf(lc, "nexmo.go complete verification, error while making request by url %s with body %s for userId [%s] : %v", url, string(bodyStr), userInfo.UserId, err)
		return false, InternalServerError
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		anlogger.Errorf(lc, "nexmo.go : complete verification, error reading response body from Nexmo, request id [%s] and code [%s] for userId [%s] : %v", userInfo.VerifyRequestId, verificationCode, userInfo.UserId, err)
		return false, InternalServerError
	}

	anlogger.Debugf(lc, "nexmo.go : complete verification, receive response from Nexmo, body=%s, request id [%s] and code [%s] for userId [%s]", string(body), userInfo.VerifyRequestId, verificationCode, userInfo.UserId)

	var nexmoResp map[string]interface{}
	err = json.Unmarshal(body, &nexmoResp)
	if err != nil {
		anlogger.Errorf(lc, "nexmo.go : complete verification, error marshaling Nexmo response, request id [%s] and code [%s] for userId [%s] : %v", userInfo.VerifyRequestId, verificationCode, userInfo.UserId, err)
		return false, InternalServerError
	}

	if status, ok := nexmoResp["status"]; ok {
		if statusStr, ok := status.(string); ok {
			anlogger.Debugf(lc, "nexmo.go : complete verification, Nexmo return status [%s] for complete verification, request id [%s] and code [%s] for userId [%s]", status, userInfo.VerifyRequestId, verificationCode, userInfo.UserId)
			var errorText string
			switch statusStr {
			case "0":
				anlogger.Debugf(lc, "nexmo.go : complete verification request was successfully finished, response %v, for userId [%s]", nexmoResp, userInfo.UserId)
				return true, ""
			case "3":
				errorText = nexmoResp["error_text"].(string)
				anlogger.Errorf(lc, "nexmo.go : complete verification error, invalid value of parameter, error text [%s], response %v for userId [%s]", errorText, nexmoResp, userInfo.UserId)
				//todo:make difference between code and numbers error
				//return "", false, CountryCallingCodeClientError
				return false, PhoneNumberClientError
			case "16":
				errorText = nexmoResp["error_text"].(string)
				anlogger.Errorf(lc, "nexmo.go : complete verification error, invalid verification code, error text [%s], response %v for userId [%s]", errorText, nexmoResp, userInfo.UserId)
				return false, WrongVerificationCodeClientError
			default:
				errorText = nexmoResp["error_text"].(string)
				anlogger.Errorf(lc, "nexmo.go : complete verification error, error text [%s], response %v for userId [%s]", errorText, nexmoResp, userInfo.UserId)
				return false, InternalServerError
			}
		}
	}

	anlogger.Errorf(lc, "nexmo.go : complete verification error, nexmo response does not contain required field [status], response %v for userId [%s]", nexmoResp, userInfo.UserId)
	return false, InternalServerError
}
