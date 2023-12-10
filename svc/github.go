package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v57/github"
)

func InitGithubClients(InstallationId int64) (*github.Client, *github.Client, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(githubPrivateKeyBase64)

	if err != nil {
		fmt.Println("Error decoding base64:", err)
		return nil, nil, err
	}

	itrForJWT, err := ghinstallation.NewAppsTransport(http.DefaultTransport, githubAppId, decodedBytes)
	itr, err := ghinstallation.New(http.DefaultTransport, githubAppId, InstallationId, decodedBytes)

	if err != nil {
		fmt.Println("Error in gh installation", err)
		return nil, nil, err
	}

	jwtClient := github.NewClient(&http.Client{Transport: itrForJWT})
	client := github.NewClient(&http.Client{Transport: itr})

	return jwtClient, client, err
}

func getInstallation(c context.Context, client *github.Client, installationId int64) (*github.Installation, *github.Response, error) {
	return client.Apps.GetInstallation(c, installationId)
}

func getInstallationRepos(c context.Context, client *github.Client) (*github.ListRepositories, *github.Response, error) {
	return client.Apps.ListRepos(c, nil)
}

func getActionsRegistrationToken(c context.Context, client *github.Client, owner, repo string) (*github.RegistrationToken, *github.Response, error) {
	return client.Actions.CreateRegistrationToken(c, owner, repo)
}
