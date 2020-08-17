package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/steinfletcher/apitest"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/hiconvo/api/clients/db"
	"github.com/hiconvo/api/clients/search"
	"github.com/hiconvo/api/random"
	"github.com/hiconvo/api/testutil"
)

var (
	_ctx          context.Context
	_handler      http.Handler
	_dbClient     db.Client
	_mongoClient  *mongo.Client
	_searchClient search.Client
)

func TestMain(m *testing.M) {
	_ctx = context.Background()
	_dbClient = testutil.NewDBClient(_ctx)
	_mongoClient, _ = testutil.NewMongoClient(_ctx)
	_searchClient = testutil.NewSearchClient()
	_handler = testutil.Handler(_dbClient, _mongoClient, _searchClient)

	testutil.ClearDB(_ctx, _dbClient)

	result := m.Run()

	testutil.ClearDB(_ctx, _dbClient)

	// _dbClient.Close()
	// closer()

	os.Exit(result)
}

func Test404(t *testing.T) {
	apitest.New().
		Handler(_handler).
		Get(fmt.Sprintf("/%s", random.String(10))).
		Expect(t).
		Status(http.StatusNotFound).
		Body(`{"message":"Not found"}`).
		End()
}

func Test415(t *testing.T) {
	apitest.New().
		Handler(_handler).
		Post("/users").
		Expect(t).
		Status(http.StatusUnsupportedMediaType).
		End()
}
