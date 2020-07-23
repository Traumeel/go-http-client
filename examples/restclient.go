package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	cl "github.com/Traumeel/go-http-client"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
)

const (
	GroupApiV1Path = "/api/v1/groups"
	UserApiV1Path  = "/api/v1/users"
)

func main() {
	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasPrefix(req.URL.String(), GroupApiV1Path):
			switch req.Method {
			case http.MethodDelete:
				fmt.Printf("deleting group: %v\n", req.URL.String())
				rw.WriteHeader(http.StatusCreated)
				return
			case http.MethodGet:
				fmt.Printf("get groups: %v\n", req.URL.String())
				data := []*Group{{
					Id:          "2",
					Name:        "Foo",
					Description: "desc2",
				}, {
					Id:          "1",
					Name:        "Bar",
					Description: "desc1",
				}}

				d, err := json.Marshal(data)
				if err != nil {
					log.Fatalf("failed to marshal: %v", err)
				}

				rw.Write(d)
				return
			}
		case req.URL.String() == UserApiV1Path:
			fmt.Printf("create user: %v\n", req.URL.String())
			data := &User{
				Id:   "1",
				Name: "name",
				Age:  20,
			}

			d, err := json.Marshal(data)
			if err != nil {
				log.Fatalf("failed to marshal: %v", err)
			}

			rw.Write(d)
			return
		}
		rw.WriteHeader(http.StatusNotFound)
	}))
	// Close the server when test finishes
	defer server.Close()

	c := NewMyApiClient(server.URL,
		cl.WithDebug(true),
		cl.WithRequestOptions(func(req *http.Request) (e error) {
		req.Header.Add("User-Agent", "my-api-client")
		return
	}))
	err := c.V1().Group().DeleteGroup("test")
	if err != nil {
		log.Fatalf("filed delete group: %v", err)
	}

	gs, err := c.V1().Group().ListGroups()
	if err != nil {
		log.Fatalf("filed to list groups: %v", err)
	}

	for _, g := range gs {
		fmt.Printf("%+v\n", g)
	}
}

func NewMyApiClient(endpoint string, options ...cl.Option) MyApiClient {
	libCl := cl.NewClient(endpoint, options...)
	c := &myApiClient{
		v1: NewV1Client(libCl),
	}

	return c
}

func NewV1Client(c *cl.Client) V1Client {
	return &v1Client{
		group: NewGroupV1Client(c),
		user:  NewUserV1Client(c),
	}
}

type MyApiClient interface {
	V1() V1Client
}

type myApiClient struct {
	v1 V1Client
}

func (a *myApiClient) V1() V1Client {
	return a.v1
}

type V1Client interface {
	Group() GroupV1Client
	User() UserV1Client
}

type v1Client struct {
	group GroupV1Client
	user  UserV1Client
}

func (c *v1Client) Group() GroupV1Client {
	return c.group
}

func (c *v1Client) User() UserV1Client {
	return c.user
}

type Group struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type GroupV1Client interface {
	ListGroups() ([]*Group, error)
	ListGroupsContext(ctx context.Context) ([]*Group, error)
	DeleteGroup(group string) error
	DeleteGroupContext(ctx context.Context, group string) error
}

type groupV1Client struct {
	*cl.Client
}

func NewGroupV1Client(cl *cl.Client) GroupV1Client {
	return &groupV1Client{cl}
}

func (api *groupV1Client) DeleteGroup(group string) error {
	return api.DeleteGroupContext(context.Background(), group)
}

func (api *groupV1Client) DeleteGroupContext(ctx context.Context, group string) error {
	values := url.Values{
		"id": {group},
	}

	if err := api.DoRequest(ctx, http.MethodDelete, GroupApiV1Path, cl.WithQueryOpt(values)); err != nil {
		return err
	}
	return nil
}

func (api *groupV1Client) ListGroups() ([]*Group, error) {
	return api.ListGroupsContext(context.Background())
}

func (api *groupV1Client) ListGroupsContext(ctx context.Context) ([]*Group, error) {
	var groups []*Group
	err := api.GetJson(ctx, GroupApiV1Path, &groups)
	if err != nil {
		return nil, err
	}

	return groups, nil
}

type User struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type UserV1Client interface {
	CreateUser(req *User) (*User, error)
	CreateUserContext(ctx context.Context, req *User) (*User, error)
}

type userV1Client struct {
	*cl.Client
}

func NewUserV1Client(cl *cl.Client) UserV1Client {
	return &userV1Client{cl}
}

func (api *userV1Client) CreateUser(req *User) (*User, error) {
	return api.CreateUserContext(context.Background(), req)
}

func (api *userV1Client) CreateUserContext(ctx context.Context, req *User) (*User, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header, 0)
	headers.Add("Content-Type", "application/json")
	headers.Add("Accept", "application/json")

	var user *User
	if err := api.DoRequestJson(ctx, http.MethodPost, UserApiV1Path, user,
		cl.WithBodyOpt(bytes.NewBuffer(data)),
		cl.WithHeadersOpt(headers)); err != nil {
		return nil, err
	}
	return user, nil
}
