package fiber

import "testing"

func Test_Route(t *testing.T) {
	dummyHandler := testEmptyHandler

	app := New()

	app.Route("/:john?/:doe?").
		Connect(dummyHandler).
		Put(dummyHandler).
		Post(dummyHandler).
		Delete(dummyHandler).
		Head(dummyHandler).
		Patch(dummyHandler).
		Options(dummyHandler).
		Trace(dummyHandler).
		Get(dummyHandler).
		All(dummyHandler).
		Use(dummyHandler)

	testStatus200(t, app, "/john/doe", MethodConnect)
	testStatus200(t, app, "/john/doe", MethodPut)
	testStatus200(t, app, "/john/doe", MethodPost)
	testStatus200(t, app, "/john/doe", MethodDelete)
	testStatus200(t, app, "/john/doe", MethodHead)
	testStatus200(t, app, "/john/doe", MethodPatch)
	testStatus200(t, app, "/john/doe", MethodOptions)
	testStatus200(t, app, "/john/doe", MethodTrace)
	testStatus200(t, app, "/john/doe", MethodGet)
	testStatus200(t, app, "/john/doe", MethodPost)
	testStatus200(t, app, "/john/doe", MethodGet)
}
