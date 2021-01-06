package main

import "encoding/json"

type loginUserReturnStruct struct {
	UserID       int    `json:"userId"`
	UserName     string `json:"username"`
	Jwt          string `json:"jwt"`
	SessionToken string `json:"sessionToken"`
	ValidUntil   string `json:"validUntil"`
}
type userLoginStruct struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

type authInfoReturnStruct struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

func getAuthInfo(serverAdress string, jwt string) authInfoReturnStruct {
	type successfulResponse struct {
		Err  bool                 `json:"err"`
		Data authInfoReturnStruct `json:"data"`
	}
	response, errList := sendAPIRequest(serverAdress, "v1/auth/info", nil, "GET", &jwt)
	if errList != nil {
		if containsErrorCode("ERR_FORBIDDEN", errList) {
			exitWithError("Forbidden. Maybe the login is expired? Try to log in again")
		} else {
			panic(errList[0].Error())
		}
	}
	var unmarshaledResponse successfulResponse
	err := json.Unmarshal(response, &unmarshaledResponse)
	if err != nil {
		panic(err)
	}
	return unmarshaledResponse.Data
}

func loginUser(serverAdress string, userName string, userPassword string) loginUserReturnStruct {
	type succesfulResponse struct {
		Data  loginUserReturnStruct `json:"data"`
		Error bool                  `json:"error"`
	}
	response, errList := sendAPIRequest(serverAdress, "v1/auth/login", userLoginStruct{
		UserName: userName,
		Password: userPassword,
	}, "POST", nil)
	if errList != nil {
		if containsErrorCode("ERR_NOT_FOUND", errList) {
			exitWithError("User not found")
		} else if containsErrorCode("ERR_FORBIDDEN", errList) {
			exitWithError("Login disallowed. Wrong password?")
		} else {
			panic(errList[0])
		}
	}
	var unmarshaledResponse succesfulResponse
	err := json.Unmarshal(response, &unmarshaledResponse)
	if err != nil {
		panic(err)
	}
	return unmarshaledResponse.Data
}

func registerUser(serverAdress string, userName string, userPassword string) string {
	type successfulResposeData struct {
		Username string `json:"username"`
	}
	type successfulRespose struct {
		Err  bool                  `json:"err"`
		Data successfulResposeData `json:"data"`
	}
	response, errList := sendAPIRequest(serverAdress, "v1/auth/register", userLoginStruct{
		UserName: userName,
		Password: userPassword,
	}, "POST", nil)
	if errList != nil {
		if containsErrorCode("ERR_INTERNAL", errList) {
			exitWithError("Internal Server error. Maybe this has something to do with\n>https://github.com/Cardsity/issue-tracker/issues/3")
		} else if containsErrorCode("ERR_PASSWORD_REQUIREMENTS_NOT_MET", errList) {
			exitWithError("Password to weak")
		} else if containsErrorCode("ERR_DUPLICATE_USERNAME", errList) {
			exitWithError("A user with that username already exists")
		} else {
			panic(errList[0].Error())
		}
	}
	var unmarshaledResponse successfulRespose
	err := json.Unmarshal(response, &unmarshaledResponse)
	if err != nil {
		panic(err)
	}
	return unmarshaledResponse.Data.Username
}
