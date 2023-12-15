package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v57/github"
	"net/http"
)

type ClientApi interface {
	GetInstallation(context.Context, int64) (*github.Installation, *github.Response, error)
	GetInstallationRepos(context.Context) (*github.ListRepositories, *github.Response, error)
	GetActionsRegistrationToken(context.Context, string, string) (*github.RegistrationToken, *github.Response, error)
}

// Client implements ClientApi interface
type Client struct {
	installationID int64
	JWT            *github.Client
	REG            *github.Client
}

func NewClient(appId int64, installationId int64, githubPrivateKeyBase64 string) (*Client, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(githubPrivateKeyBase64)

	if err != nil {
		fmt.Println("Error decoding base64:", err)
		return nil, err
	}

	jwtTransport, err := ghinstallation.NewAppsTransport(http.DefaultTransport, appId, decodedBytes)
	regularTransport, err := ghinstallation.New(http.DefaultTransport, appId, installationId, decodedBytes)
	if err != nil {
		fmt.Println("Error in gh installation", err)
		return nil, err
	}

	return &Client{
		JWT:            github.NewClient(&http.Client{Transport: jwtTransport}),
		REG:            github.NewClient(&http.Client{Transport: regularTransport}),
		installationID: installationId,
	}, nil
}

func (cl Client) GetInstallation() (*github.Installation, *github.Response, error) {
	return cl.JWT.Apps.GetInstallation(context.Background(), cl.installationID)
}

func (cl Client) GetInstallationRepos() (*github.ListRepositories, *github.Response, error) {
	return cl.REG.Apps.ListRepos(context.Background(), nil)
}

func (cl Client) GetActionsRegistrationToken(owner string, repo string) (*github.RegistrationToken, *github.Response, error) {
	return cl.REG.Actions.CreateRegistrationToken(context.Background(), owner, repo)
}
