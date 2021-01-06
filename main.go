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
	"strings"
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
	action := flag.String("a", "checkConnection", "Action (login, register, authInfo, checkConnection)")
	userName := flag.String("u", "", "Username (for registration / login)")
	userPassword := flag.String("p", "", "Password (for registration / login)")
	flag.Parse()

	// Check whether the given combination of arguments is valid
	*action = strings.ToLower(*action)
	actionIsValid := false
	for _, v := range []string{"login", "register", "checkConnection", "authinfo"} {
		if *action == v {
			actionIsValid = true
		}
	}
	if !actionIsValid {
		exitWithError("Unknown action")
	}
	if *action == "login" || *action == "register" {
		if *userName == "" || *userPassword == "" {
			exitWithError("Please set a username and a password")
		}
	} else if *jwt == "" {
		exitWithError("Please set a jwt using the config.json or the command line argument -j")
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

type erroredReturnStruct struct {
	Error  bool     `json:"error"`
	Errors []string `json:"errors"`
}

func sendAPIRequest(serverAdress string, apiCall string, request interface{}, method string, jwt *string) ([]byte, []error) {
	var requestToSend *bytes.Buffer = new(bytes.Buffer)
	if request != nil {
		marshaledRequest, err := json.Marshal(request)
		if err != nil {
			panic(err)
		}
		requestToSend = bytes.NewBuffer(marshaledRequest)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("%v/%v", serverAdress, apiCall), requestToSend)
	if jwt != nil {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer JWT %v", *jwt))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, []error{errors.New("Connection failed")}
	}
	defer res.Body.Close()
	response, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	if res.StatusCode != 200 {
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
	case "authinfo":
		{
			authInfo := getAuthInfo(properties.config.ServerAdress, properties.config.JWT)
			fmt.Printf("Logged in as %v (%v)\n", authInfo.Username, authInfo.ID)
		}
	}
}
