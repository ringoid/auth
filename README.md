# auth service

### STAGE API ENDPOINT IS ``mstyzyhb69.execute-api.eu-west-1.amazonaws.com``
### PROD API ENDPOINT IS ````

### Start auth

* url ``https://{API ENDPOINT}/Prod/start_verification``

POST request

Headers:

* X-Ringoid-Android-BuildNum : 1       //int, X-Ringoid-iOS-BuildNum in case of iOS
* Content-Type : application/json

Body:

    {
        "countryCallingCode":7,
        "phone":"9211234567",
        "dtTC":1535120929, //unix time when Terms and Conditions were accepted
        "dtLA":1535120929, //unix time when Privacy Notes were accepted
        "dtPN":1535120929, //unix time when Legal age was confirmed
        "locale":"en",
        "clientValidationFail":true,
        "deviceModel":"device model info",
        "osVersion":"version of os"
    }
    
    all parameters are required
    
 Locale values could be found [here](https://www.twilio.com/docs/verify/supported-languages)
    
 Response Body:
 
    {
        "errorCode":"",
        "errorMessage":"",
        "sessionId":"sdfsdf-fsdf-fsd",
        "customerId":"ksjdhfha-asff"
    }
    
Possible errorCodes:

* InternalServerError
* WrongRequestParamsClientError
* PhoneNumberClientError
* CountryCallingCodeClientError
* TooOldAppVersionClientError

### Complete auth

* url ``https://{API ENDPOINT}/Prod/complete_verification``

POST request

Headers:

* X-Ringoid-Android-BuildNum : 1       //int, X-Ringoid-iOS-BuildNum in case of iOS
* Content-Type : application/json

Body:

    {
        "sessionId":"sdkjfhh-dfsdf-e333",
        "verificationCode":"6121"
    }
    
    all parameters are required
    
 Response Body:
 
    {
        "accessToken":"aslkdjflkjh-sdfasdfsadf-dd",
        "accountAlreadyExist":false,
        "errorCode":"",
        "errorMessage":""
    }
    
Possible errorCodes:

* InternalServerError
* WrongRequestParamsClientError
* WrongSessionIdClientError
* NoPendingVerificationClientError
* WrongVerificationCodeClientError
* TooOldAppVersionClientError

### Create user profile

* url ``https://{API ENDPOINT}/Prod/create_profile``

POST request

Headers:

* X-Ringoid-Android-BuildNum : 1       //int, X-Ringoid-iOS-BuildNum in case of iOS
* Content-Type : application/json

Body:

    {
        "accessToken":"adasdasd-fadfs-sdffd",
        "yearOfBirth":1982,
        "sex":"male" // possible values are **male** or **female** 
    }
    
    all parameters are required
    
 Response Body:
 
    {
        "errorCode":"",
        "errorMessage":""
    }
    
Possible errorCodes:

* InternalServerError
* WrongYearOfBirthClientError
* WrongSexClientError
* WrongRequestParamsClientError
* InvalidAccessTokenClientError
* TooOldAppVersionClientError

### Update user's settings

* url ``https://{API ENDPOINT}/Prod/update_settings``

POST request

Headers:

* X-Ringoid-Android-BuildNum : 1       //int, X-Ringoid-iOS-BuildNum in case of iOS
* Content-Type : application/json

Body:

    {
        "accessToken":"adasdasd-fadfs-sdffd",
        "safeDistanceInMeter":0,
        "pushMessages":true,
        "pushMatches":true,
        "pushLikes":"EVERY" //possible values NONE/EVERY/10_NEW/100_NEW 
    }
    
    all parameters are required
    
 Response Body:
 
    {
        "errorCode":"",
        "errorMessage":""
    }
    
Possible errorCodes:

* InternalServerError
* WrongRequestParamsClientError
* InvalidAccessTokenClientError
* TooOldAppVersionClientError

### Logout user

* url ``https://{API ENDPOINT}/Prod/logout``

POST request

Headers:

* X-Ringoid-Android-BuildNum : 1       //int, X-Ringoid-iOS-BuildNum in case of iOS
* Content-Type : application/json

Body

    {
        "accessToken":"adasdasd-fadfs-sdffd"
    }

    
 Response Body:
 
    {
        "errorCode":"",
        "errorMessage":""
    }
    
Possible errorCodes:

* InternalServerError
* WrongRequestParamsClientError
* InvalidAccessTokenClientError
* TooOldAppVersionClientError

### Get user's settings

* url ``https://{API ENDPOINT}/Prod/get_settings?accessToken={ACCESS TOKEN}``

GET request

Headers:

* X-Ringoid-Android-BuildNum : 1       //int, X-Ringoid-iOS-BuildNum in case of iOS
* Content-Type : application/json

 Response Body:
 
    {
        "errorCode":"",
        "errorMessage":"",
        "whoCanSeePhoto":"OPPOSITE", 
        "safeDistanceInMeter":0,
        "pushMessages":true,
        "pushMatches":true,
        "pushLikes":"EVERY"
    }
    
Possible errorCodes:

* InternalServerError
* WrongRequestParamsClientError
* InvalidAccessTokenClientError
* TooOldAppVersionClientError

## Analytics Events

1. AUTH_USER_ACCEPT_TERMS

* userId - string
* sourceIp - string
* unixTime - int
* locale - string
* clientValidationFail - was phone number's validation failed on client side
* dtTC - date and time when Terms and conditions were accepted
* dtPN - date and time when Privacy Notes were accepted
* dtLA - date and time when Legal age was confirmed
* deviceModel - string (model of the device (Build.MODEL + "," + Build.MANUFACTURER + "," + Build.PRODUCT))
* osVersion - string
* eventType - string (AUTH_USER_ACCEPT_TERMS)

`{"deviceModel":"device model","osVersion":"2.2","userId":"aslkdl-asfmfa-asd","sourceIp":"82.102.27.75","unixTime":1534338646,"dtTC":1535120929,"dtPN":1535120929,"dtLA":1535120929,locale":"","clientValidationFail":true,"eventType":"AUTH_USER_ACCEPT_TERMS"}`

2. AUTH_USER_START_VERIFICATION

* userId - string
* countryCode - int
* verifyProvider - string
* locale - string
* unixTime - int
* eventType - string (AUTH_USER_START_VERIFICATION)

`{"userId":"aslkdl-asfmfa-asd","countryCode":7,"verifyProvider":"Nexmo","locale":"ru","unixTime":1534338646,"eventType":"AUTH_USER_START_VERIFICATION"}`


3. AUTH_USER_COMPLETE_VERIFICATION

* userId - string
* countryCode - int
* verifyProvider - string
* verificationStartAt - int
* locale - string
* unixTime - int
* eventType - string (AUTH_USER_COMPLETE_VERIFICATION)

`{"userId":"aslkdl-asfmfa-asd","countryCode":7,"verifyProvider":"Nexmo","verificationStartAt":123123123123,"locale":"ru","unixTime":1534338646,"eventType":"AUTH_USER_COMPLETE_VERIFICATION"}`

4. AUTH_USER_PROFILE_CREATED

* userId - string
* sex - string
* yearOfBirth - int
* unixTime - int
* eventType - string (AUTH_USER_PROFILE_CREATED)

`{"userId":"aslkdl-asfmfa-asd","sex":"male","yearOfBirth":"1982","unixTime":1534338646,"eventType":"AUTH_USER_PROFILE_CREATED"}`

5. AUTH_USER_SETTINGS_UPDATED

* userId - string
* safeDistanceInMeter - int
* pushMessages - bool
* pushMatches - bool
* pushLikes - string
* unixTime - int
* eventType - string (AUTH_USER_SETTINGS_UPDATED)

`{"userId":"aslkdl-asfmfa-asd","safeDistanceInMeter":0,"pushMessages":true,"pushMatches":true,"pushLikes":"EVERY","unixTime":1534338646, "eventType":"AUTH_USER_SETTINGS_UPDATED"}`

6. AUTH_USER_LOGOUT

* userId - string
* unixTime - int
* eventType - string (AUTH_USER_LOGOUT)

`{"userId":"aslkdl-asfmfa-asd", "unixTime":1534338646, "eventType":"AUTH_USER_LOGOUT"}`