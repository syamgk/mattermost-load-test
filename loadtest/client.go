// Copyright (c) 2017 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information

package loadtest

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/mattermost/mattermost-load-test/cmdlog"
	"github.com/mattermost/platform/model"
)

func newV3ClientFromToken(token string, serverUrl string) *model.Client {
	client := model.NewClient(serverUrl)
	client.AuthToken = token
	client.AuthType = model.HEADER_BEARER
	return client
}

func newClientFromToken(token string, serverUrl string) *model.Client4 {
	client := model.NewAPIv4Client(serverUrl)
	client.AuthToken = token
	client.AuthType = model.HEADER_BEARER
	return client
}

func loginAsUsers(cfg *LoadTestConfig) []string {
	tokens := make([]string, cfg.UserEntitiesConfiguration.NumActiveEntities)

	ThreadSplit(cfg.UserEntitiesConfiguration.NumActiveEntities, runtime.GOMAXPROCS(0)*2, PrintCounter, func(usernum int) {
		client := model.NewClient(cfg.ConnectionConfiguration.ServerURL)
		if _, err := client.Login("success+user"+strconv.Itoa(usernum)+"@simulator.amazonses.com", "Loadtestpassword1"); err != nil {
			cmdlog.Errorf("Unable to login as user %v", usernum)
		}
		tokens[usernum] = client.AuthToken
	})

	return tokens
}

func getAdminClient(serverURL string, adminEmail string, adminPass string, cmdrun ServerCommandRunner) *model.Client4 {
	client := model.NewAPIv4Client(serverURL)

	if success, resp := client.GetPing(); resp.Error != nil || success != "OK" {
		cmdlog.Errorf("Failed to ping server at %v", serverURL)
		if success != "" {
			cmdlog.Errorf("Got %v from ping", success)
		}
		cmdlog.Error("Did you follow the setup guide and modify loadtestconfig.json?")
		cmdlog.AppError(resp.Error)
		return nil
	} else {
		cmdlog.Infof("Successfully pinged server at %v", serverURL)
	}

	var adminUser *model.User
	if user, _ := client.Login(adminEmail, adminPass); user == nil {
		cmdlog.Info("Failed to login as admin user.")
		cmdlog.Info("Attempting to create admin user.")
		if success, output := cmdrun.RunPlatformCommand(fmt.Sprintf("user create --email %v --password %v --system_admin --username ltadmin", adminEmail, adminPass)); !success {
			cmdlog.Errorf("Failed to create admin user. Got: %v", output)
			return nil
		}
		if success, output := cmdrun.RunPlatformCommand(fmt.Sprintf("user verify ltadmin")); !success {
			cmdlog.Errorf("Failed to verify email of admin user. Got: %v", output)
			return nil
		}
		if user2, resp2 := client.Login(adminEmail, adminPass); user2 == nil {
			cmdlog.Errorf("Failed to login to successfully created admin account. %v", resp2.Error.Error())
			cmdlog.AppError(resp2.Error)
			return nil
		} else {
			adminUser = user2
		}
	} else {
		adminUser = user
	}

	cmdlog.Infof("Successfully logged in with user %v and roles of %v", adminUser.Email, adminUser.Roles)

	if !adminUser.IsInRole(model.PERMISSIONS_SYSTEM_ADMIN) {
		cmdlog.Errorf("%v is not a system admin, please run the command", adminUser.Email)
		cmdlog.Errorf("'./bin/platform roles system_admin %v", adminUser.Username)
		return nil
	}

	// Wait here because somtimes we are too fast in making our first request
	time.Sleep(time.Second)

	return client
}
