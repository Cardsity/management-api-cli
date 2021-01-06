package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type configProperties struct {
	ServerAdress string
	JWT          string
}

type runtimeProperties struct {
	config               configProperties
	action               string
	userActionProperties userActionProperties
}

type userActionProperties struct {
	userName     string
	userPassword string
}

func containsErrorCode(a string, list []error) bool {
	for _, b := range list {
		if b.Error() == a {
			return true
		}
	}
	return false
}

func exitWithError(err string) {
	fmt.Println(err)
	os.Exit(1)
}

func readConfiguration() configProperties {
	open, err := ioutil.ReadFile("config.json")
	// Default configuration values
	unmarshaled := configProperties{
		ServerAdress: "http://127.0.0.1:5000",
	}
	// If the configuration file doesn't exist, return the default values
	if os.IsNotExist(err) {
		return unmarshaled
	}
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(open, &unmarshaled)
	if err != nil {
		panic(err)
	}
	return unmarshaled
}
func getProperties() runtimeProperties {
	readConfig := readConfiguration()
	// Values passed via command line args take precedence over values passed via config.json
	serverAdress := flag.String("s", readConfig.ServerAdress, "Specify the server adress")
	jwt := flag.String("j", readConfig.JWT, "Specify the bearer token")
	action := flag.String("a", "checkConnection", "Action (login, register, checkConnection)")
	userName := flag.String("u", "", "Username (for registration / login)")
	userPassword := flag.String("p", "", "Password (for registration / login)")
	flag.Parse()

	// Check whether the given combination of arguments is valid
	actionIsValid := false
	for _, v := range []string{"login", "register", "checkConnection"} {
		if *action == v {
			actionIsValid = true
		}
	}
	if !actionIsValid {
		panic("Unknown action")
	}
	if *action == "login" || *action == "register" {
		if *userName == "" || *userPassword == "" {
			panic("Please set a username and a password")
		}
	} else if *jwt == "" {
		panic("Please set a jwt using the config.json or the command line argument -j")
	}

	// Return the valid conclusion
	return runtimeProperties{
		config: configProperties{
			ServerAdress: *serverAdress,
			JWT:          *jwt,
		},
		action: *action,
		userActionProperties: userActionProperties{
			userName:     *userName,
			userPassword: *userPassword,
		},
	}
}

type loginUserReturnStruct struct {
	UserID       int    `json:"userId"`
	UserName     string `json:"username"`
	Jwt          string `json:"jwt"`
	SessionToken string `json:"sessionToken"`
	ValidUntil   string `json:"validUntil"`
}

type erroredReturnStruct struct {
	Error  bool     `json:"error"`
	Errors []string `json:"errors"`
}

func sendAPIRequest(serverAdress string, apiCall, request interface{}) ([]byte, []error) {
	marshaledRequest, err := json.Marshal(request)
	if err != nil {
		panic(err)
	}
	req, err := http.Post(fmt.Sprintf("%v/%v", serverAdress, apiCall), "application/json", bytes.NewBuffer(marshaledRequest))
	if err != nil {
		return nil, []error{errors.New("Connection failed")}
	}
	defer req.Body.Close()
	response, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	if req.StatusCode != 200 {
		var errorStruct erroredReturnStruct
		err = json.Unmarshal(response, &errorStruct)
		if err != nil {
			panic(err)
		}
		var errorList []error
		for _, v := range errorStruct.Errors {
			errorList = append(errorList, errors.New(v))
		}
		return response, errorList
	}
	return response, nil
}

type userLoginStruct struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

func loginUser(serverAdress string, userName string, userPassword string) loginUserReturnStruct {
	type succesfulResponse struct {
		Data  loginUserReturnStruct `json:"data"`
		Error bool                  `json:"error"`
	}
	response, errList := sendAPIRequest(serverAdress, "v1/auth/login", userLoginStruct{
		UserName: userName,
		Password: userPassword,
	})
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
	})
	if errList != nil {
		if containsErrorCode("ERR_INTERNAL", errList) {
			exitWithError("Internal Server error. Maybe this has something to do with\n>https://github.com/Cardsity/issue-tracker/issues/3")
		} else if containsErrorCode("ERR_PASSWORD_REQUIREMENTS_NOT_MET", errList) {
			exitWithError("Password to weak")
		} else {
			panic(errList)
		}
	}
	var unmarshaledResponse successfulRespose
	err := json.Unmarshal(response, &unmarshaledResponse)
	if err != nil {
		panic(err)
	}
	return unmarshaledResponse.Data.Username
}

func writeToConfiguration(properties configProperties) {
	marshaled, err := json.MarshalIndent(properties, "", "    ")
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile("config.json", marshaled, 0660)
	if err != nil {
		panic(err)
	}
}

func main() {
	properties := getProperties()
	// Check whether the management api is reachable
	isReachable, err := http.Get(fmt.Sprintf("%v/v1/reachable", properties.config.ServerAdress))
	if err != nil || isReachable.StatusCode != 200 {
		panic(fmt.Sprintf("Management API not reachable: %v", err))
	}
	defer isReachable.Body.Close()
	switch properties.action {
	case "checkConnection":
		{
			fmt.Println("Connection successful")
		}
	case "login":
		{
			loggedIn := loginUser(properties.config.ServerAdress, properties.userActionProperties.userName, properties.userActionProperties.userPassword)
			properties.config.JWT = loggedIn.Jwt
			writeToConfiguration(properties.config)
			fmt.Printf(
				"Logged in as %v (%v)\n"+
					"JWT: %v \n"+
					"SessionToken: %v\n"+
					"The login expires on %v\n"+
					"(login saved to config.json)\n",
				loggedIn.UserName, loggedIn.UserID, loggedIn.Jwt, loggedIn.SessionToken, loggedIn.ValidUntil,
			)
		}
	case "register":
		{
			createdUserName := registerUser(properties.config.ServerAdress, properties.userActionProperties.userName, properties.userActionProperties.userPassword)
			fmt.Printf("Created user with name %v\n", createdUserName)
		}
	}
}
