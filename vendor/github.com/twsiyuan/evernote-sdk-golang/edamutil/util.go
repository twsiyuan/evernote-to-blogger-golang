package edamutil

import (
	"fmt"

	"git.apache.org/thrift.git/lib/go/thrift"
	"github.com/twsiyuan/evernote-sdk-golang/edam"
)

type EnvironmentType int

const (
	SANDBOX EnvironmentType = iota
	PRODUCTION
)

func Host(envType EnvironmentType) string {
	host := "www.evernote.com"
	if envType == SANDBOX {
		host = "sandbox.evernote.com"
	}
	return host
}

func NewUserStore(envType EnvironmentType) (*edam.UserStoreClient, error) {
	url := fmt.Sprintf("https://%s/edam/user", Host(envType))
	c, err := thrift.NewTHttpPostClient(url)
	if err != nil {
		return nil, err
	}
	return edam.NewUserStoreClientFactory(
		c,
		thrift.NewTBinaryProtocolFactoryDefault(),
	), nil
}

func NewNoteStore(userstore *edam.UserStoreClient, authenticationToken string) (*edam.NoteStoreClient, error) {
	urls, err := userstore.GetUserUrls(authenticationToken)
	if err != nil {
		return nil, err
	}

	url := urls.GetNoteStoreUrl()
	c, err := thrift.NewTHttpPostClient(url)
	if err != nil {
		return nil, err
	}

	return edam.NewNoteStoreClientFactory(
		c,
		thrift.NewTBinaryProtocolFactoryDefault(),
	), nil
}
