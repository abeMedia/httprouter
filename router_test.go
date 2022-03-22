// Copyright 2013 Julien Schmidt. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package httprouter

import (
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestParams(t *testing.T) {
	ps := Params{
		Param{"param1", "value1"},
		Param{"param2", "value2"},
		Param{"param3", "value3"},
	}
	for i := range ps {
		if val := ps.ByName(ps[i].Key); val != ps[i].Value {
			t.Errorf("Wrong value for %s: Got %s; Want %s", ps[i].Key, val, ps[i].Value)
		}
	}
	if val := ps.ByName("noKey"); val != "" {
		t.Errorf("Expected empty string for not found key; got: %s", val)
	}
}

func TestRouter(t *testing.T) {
	router := New()

	routed := false
	router.Handle(http.MethodGet, "/user/:name", func(ctx *fasthttp.RequestCtx, ps Params) {
		routed = true
		want := Params{Param{"name", "gopher"}}
		if !reflect.DeepEqual(ps, want) {
			t.Fatalf("wrong wildcard values: want %v, got %v", want, ps)
		}
	})

	ctx := newContext(http.MethodGet, "/user/gopher", nil)
	router.HandleFastHTTP(ctx)

	if !routed {
		t.Fatal("routing failed")
	}
}

type handlerStruct struct {
	handled *bool
}

func (h handlerStruct) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	*h.handled = true
}

func TestRouterAPI(t *testing.T) {
	var get, head, options, post, put, patch, delete, handler, handlerFunc bool

	httpHandler := handlerStruct{&handler}

	router := New()
	router.GET("/GET", func(ctx *fasthttp.RequestCtx, _ Params) {
		get = true
	})
	router.HEAD("/GET", func(ctx *fasthttp.RequestCtx, _ Params) {
		head = true
	})
	router.OPTIONS("/GET", func(ctx *fasthttp.RequestCtx, _ Params) {
		options = true
	})
	router.POST("/POST", func(ctx *fasthttp.RequestCtx, _ Params) {
		post = true
	})
	router.PUT("/PUT", func(ctx *fasthttp.RequestCtx, _ Params) {
		put = true
	})
	router.PATCH("/PATCH", func(ctx *fasthttp.RequestCtx, _ Params) {
		patch = true
	})
	router.DELETE("/DELETE", func(ctx *fasthttp.RequestCtx, _ Params) {
		delete = true
	})
	router.Handler(http.MethodGet, "/Handler", httpHandler)
	router.HandlerFunc(http.MethodGet, "/HandlerFunc", func(w http.ResponseWriter, r *http.Request) {
		handlerFunc = true
	})

	ctx := newContext(http.MethodGet, "/GET", nil)
	router.HandleFastHTTP(ctx)
	if !get {
		t.Error("routing GET failed")
	}

	ctx = newContext(http.MethodHead, "/GET", nil)
	router.HandleFastHTTP(ctx)
	if !head {
		t.Error("routing HEAD failed")
	}

	ctx = newContext(http.MethodOptions, "/GET", nil)
	router.HandleFastHTTP(ctx)
	if !options {
		t.Error("routing OPTIONS failed")
	}

	ctx = newContext(http.MethodPost, "/POST", nil)
	router.HandleFastHTTP(ctx)
	if !post {
		t.Error("routing POST failed")
	}

	ctx = newContext(http.MethodPut, "/PUT", nil)
	router.HandleFastHTTP(ctx)
	if !put {
		t.Error("routing PUT failed")
	}

	ctx = newContext(http.MethodPatch, "/PATCH", nil)
	router.HandleFastHTTP(ctx)
	if !patch {
		t.Error("routing PATCH failed")
	}

	ctx = newContext(http.MethodDelete, "/DELETE", nil)
	router.HandleFastHTTP(ctx)
	if !delete {
		t.Error("routing DELETE failed")
	}

	ctx = newContext(http.MethodGet, "/Handler", nil)
	router.HandleFastHTTP(ctx)
	if !handler {
		t.Error("routing Handler failed")
	}

	ctx = newContext(http.MethodGet, "/HandlerFunc", nil)
	router.HandleFastHTTP(ctx)
	if !handlerFunc {
		t.Error("routing HandlerFunc failed")
	}
}

func TestRouterInvalidInput(t *testing.T) {
	router := New()

	handle := func(_ *fasthttp.RequestCtx, _ Params) {}

	recv := catchPanic(func() {
		router.Handle("", "/", handle)
	})
	if recv == nil {
		t.Fatal("registering empty method did not panic")
	}

	recv = catchPanic(func() {
		router.GET("", handle)
	})
	if recv == nil {
		t.Fatal("registering empty path did not panic")
	}

	recv = catchPanic(func() {
		router.GET("noSlashRoot", handle)
	})
	if recv == nil {
		t.Fatal("registering path not beginning with '/' did not panic")
	}

	recv = catchPanic(func() {
		router.GET("/", nil)
	})
	if recv == nil {
		t.Fatal("registering nil handler did not panic")
	}
}

func TestRouterChaining(t *testing.T) {
	router1 := New()
	router2 := New()
	router1.NotFound = router2.HandleFastHTTP

	fooHit := false
	router1.POST("/foo", func(ctx *fasthttp.RequestCtx, _ Params) {
		fooHit = true
		ctx.SetStatusCode(http.StatusOK)
	})

	barHit := false
	router2.POST("/bar", func(ctx *fasthttp.RequestCtx, _ Params) {
		barHit = true
		ctx.SetStatusCode(http.StatusOK)
	})

	ctx := newContext(http.MethodPost, "/foo", nil)
	router1.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusOK && fooHit) {
		t.Errorf("Regular routing failed with router chaining.")
		t.FailNow()
	}

	ctx = newContext(http.MethodPost, "/bar", nil)
	router1.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusOK && barHit) {
		t.Errorf("Chained routing failed with router chaining.")
		t.FailNow()
	}

	ctx = newContext(http.MethodPost, "/qax", nil)
	router1.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusNotFound) {
		t.Errorf("NotFound behavior failed with router chaining.")
		t.FailNow()
	}
}

func BenchmarkAllowed(b *testing.B) {
	handlerFunc := func(ctx *fasthttp.RequestCtx, _ Params) {}

	router := New()
	router.POST("/path", handlerFunc)
	router.GET("/path", handlerFunc)

	b.Run("Global", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = router.allowed("*", http.MethodOptions)
		}
	})
	b.Run("Path", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = router.allowed("/path", http.MethodOptions)
		}
	})
}

func TestRouterOPTIONS(t *testing.T) {
	handlerFunc := func(ctx *fasthttp.RequestCtx, _ Params) {}

	router := New()
	router.POST("/path", handlerFunc)

	// test not allowed
	// * (server)
	ctx := newContext(http.MethodOptions, "*", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	} else if allow := b2s(ctx.Response.Header.Peek("Allow")); allow != "OPTIONS, POST" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// path
	ctx = newContext(http.MethodOptions, "/path", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	} else if allow := b2s(ctx.Response.Header.Peek("Allow")); allow != "OPTIONS, POST" {
		t.Error("unexpected Allow header value: " + allow)
	}

	ctx = newContext(http.MethodOptions, "/doesnotexist", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusNotFound) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	}

	// add another method
	router.GET("/path", handlerFunc)

	// set a global OPTIONS handler
	router.GlobalOPTIONS = func(ctx *fasthttp.RequestCtx) {
		// Adjust status code to 204
		ctx.SetStatusCode(http.StatusNoContent)
	}

	// test again
	// * (server)
	ctx = newContext(http.MethodOptions, "*", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusNoContent) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	} else if allow := b2s(ctx.Response.Header.Peek("Allow")); allow != "GET, OPTIONS, POST" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// path
	ctx = newContext(http.MethodOptions, "/path", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusNoContent) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	} else if allow := b2s(ctx.Response.Header.Peek("Allow")); allow != "GET, OPTIONS, POST" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// custom handler
	var custom bool
	router.OPTIONS("/path", func(ctx *fasthttp.RequestCtx, _ Params) {
		custom = true
	})

	// test again
	// * (server)
	ctx = newContext(http.MethodOptions, "*", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusNoContent) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	} else if allow := b2s(ctx.Response.Header.Peek("Allow")); allow != "GET, OPTIONS, POST" {
		t.Error("unexpected Allow header value: " + allow)
	}
	if custom {
		t.Error("custom handler called on *")
	}

	// path
	ctx = newContext(http.MethodOptions, "/path", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	}
	if !custom {
		t.Error("custom handler not called")
	}
}

func TestRouterNotAllowed(t *testing.T) {
	handlerFunc := func(_ *fasthttp.RequestCtx, _ Params) {}

	router := New()
	router.POST("/path", handlerFunc)

	// test not allowed
	ctx := newContext(http.MethodGet, "/path", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusMethodNotAllowed) {
		t.Errorf("NotAllowed handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	} else if allow := b2s(ctx.Response.Header.Peek("Allow")); allow != "OPTIONS, POST" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// add another method
	router.DELETE("/path", handlerFunc)
	router.OPTIONS("/path", handlerFunc) // must be ignored

	// test again
	ctx = newContext(http.MethodGet, "/path", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusMethodNotAllowed) {
		t.Errorf("NotAllowed handling failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	} else if allow := b2s(ctx.Response.Header.Peek("Allow")); allow != "DELETE, OPTIONS, POST" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// test custom handler
	responseText := "custom method"
	router.MethodNotAllowed = func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(http.StatusTeapot)
		ctx.Write([]byte(responseText))
	}
	ctx = newContext(http.MethodGet, "/path", nil)
	router.HandleFastHTTP(ctx)
	if got := b2s(ctx.Response.Body()); !(got == responseText) {
		t.Errorf("unexpected response got %q want %q", got, responseText)
	}
	if ctx.Response.StatusCode() != http.StatusTeapot {
		t.Errorf("unexpected response code %d want %d", ctx.Response.StatusCode(), http.StatusTeapot)
	}
	if allow := b2s(ctx.Response.Header.Peek("Allow")); allow != "DELETE, OPTIONS, POST" {
		t.Error("unexpected Allow header value: " + allow)
	}
}

func TestRouterNotFound(t *testing.T) {
	handlerFunc := func(ctx *fasthttp.RequestCtx, _ Params) {}

	router := New()
	router.GET("/path", handlerFunc)
	router.GET("/dir/", handlerFunc)
	router.GET("/", handlerFunc)

	testRoutes := []struct {
		route    string
		code     int
		location string
	}{
		{"/path/", http.StatusMovedPermanently, "http:///path"},   // TSR -/
		{"/dir", http.StatusMovedPermanently, "http:///dir/"},     // TSR +/
		{"", http.StatusOK, "http:///"},                           // fasthttp automatically sets empty path to `/`
		{"/PATH", http.StatusMovedPermanently, "http:///path"},    // Fixed Case
		{"/DIR/", http.StatusMovedPermanently, "http:///dir/"},    // Fixed Case
		{"/PATH/", http.StatusMovedPermanently, "http:///path"},   // Fixed Case -/
		{"/DIR", http.StatusMovedPermanently, "http:///dir/"},     // Fixed Case +/
		{"/../path", http.StatusMovedPermanently, "http:///path"}, // CleanPath
		{"/nope", http.StatusNotFound, "http://"},                 // NotFound
	}
	for _, tr := range testRoutes {
		ctx := newContext(http.MethodGet, tr.route, nil)
		router.HandleFastHTTP(ctx)
		if !(ctx.Response.StatusCode() == tr.code && (ctx.Response.StatusCode() == http.StatusNotFound || ctx.Response.StatusCode() == http.StatusOK || b2s(ctx.Response.Header.Peek("Location")) == tr.location)) {
			t.Errorf("NotFound handling route %s failed: Code=%d, Header=%v", tr.route, ctx.Response.StatusCode(), b2s(ctx.Response.Header.Peek("Location")))
		}
	}

	// Test custom not found handler
	var notFound bool
	router.NotFound = func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(http.StatusNotFound)
		notFound = true
	}
	ctx := newContext(http.MethodGet, "/nope", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusNotFound && notFound == true) {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	}

	// Test other method than GET (want 308 instead of 301)
	router.PATCH("/path", handlerFunc)
	ctx = newContext(http.MethodPatch, "/path/", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusPermanentRedirect && b2s(ctx.Response.Header.Peek("Location")) == "http:///path") {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", ctx.Response.StatusCode(), ctx.Response.Header.String())
	}

	// Test special case where no node for the prefix "/" exists
	router = New()
	router.GET("/a", handlerFunc)
	ctx = newContext(http.MethodGet, "/", nil)
	router.HandleFastHTTP(ctx)
	if !(ctx.Response.StatusCode() == http.StatusNotFound) {
		t.Errorf("NotFound handling route / failed: Code=%d", ctx.Response.StatusCode())
	}
}

func TestRouterPanicHandler(t *testing.T) {
	router := New()
	panicHandled := false

	router.PanicHandler = func(ctx *fasthttp.RequestCtx, p interface{}) {
		panicHandled = true
	}

	router.Handle(http.MethodPut, "/user/:name", func(_ *fasthttp.RequestCtx, _ Params) {
		panic("oops!")
	})

	ctx := newContext(http.MethodPut, "/user/gopher", nil)

	defer func() {
		if rcv := recover(); rcv != nil {
			t.Fatal("handling panic failed")
		}
	}()

	router.HandleFastHTTP(ctx)

	if !panicHandled {
		t.Fatal("simulating failed")
	}
}

func TestRouterLookup(t *testing.T) {
	routed := false
	wantHandle := func(ctx *fasthttp.RequestCtx, _ Params) {
		routed = true
	}
	wantParams := Params{Param{"name", "gopher"}}

	router := New()

	// try empty router first
	handle, _, tsr := router.Lookup(http.MethodGet, "/nope")
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if tsr {
		t.Error("Got wrong TSR recommendation!")
	}

	// insert route and try again
	router.GET("/user/:name", wantHandle)
	handle, params, _ := router.Lookup(http.MethodGet, "/user/gopher")
	if handle == nil {
		t.Fatal("Got no handle!")
	} else {
		handle(nil, nil)
		if !routed {
			t.Fatal("Routing failed!")
		}
	}
	if !reflect.DeepEqual(params, wantParams) {
		t.Fatalf("Wrong parameter values: want %v, got %v", wantParams, params)
	}
	routed = false

	// route without param
	router.GET("/user", wantHandle)
	handle, params, _ = router.Lookup(http.MethodGet, "/user")
	if handle == nil {
		t.Fatal("Got no handle!")
	} else {
		handle(nil, nil)
		if !routed {
			t.Fatal("Routing failed!")
		}
	}
	if params != nil {
		t.Fatalf("Wrong parameter values: want %v, got %v", nil, params)
	}

	handle, _, tsr = router.Lookup(http.MethodGet, "/user/gopher/")
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if !tsr {
		t.Error("Got no TSR recommendation!")
	}

	handle, _, tsr = router.Lookup(http.MethodGet, "/nope")
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if tsr {
		t.Error("Got wrong TSR recommendation!")
	}
}

func TestRouterParamsFromContext(t *testing.T) {
	routed := false

	wantParams := Params{Param{"name", "gopher"}}
	handlerFunc := func(_ http.ResponseWriter, req *http.Request) {
		// get params from request context
		params := ParamsFromContext(req.Context())

		if !reflect.DeepEqual(params, wantParams) {
			t.Fatalf("Wrong parameter values: want %v, got %v", wantParams, params)
		}

		routed = true
	}

	var nilParams Params
	handlerFuncNil := func(_ http.ResponseWriter, req *http.Request) {
		// get params from request context
		params := ParamsFromContext(req.Context())

		if !reflect.DeepEqual(params, nilParams) {
			t.Fatalf("Wrong parameter values: want %v, got %v", nilParams, params)
		}

		routed = true
	}
	router := New()
	router.HandlerFunc(http.MethodGet, "/user", handlerFuncNil)
	router.HandlerFunc(http.MethodGet, "/user/:name", handlerFunc)

	ctx := newContext(http.MethodGet, "/user/gopher", nil)
	router.HandleFastHTTP(ctx)
	if !routed {
		t.Fatal("Routing failed!")
	}

	routed = false
	ctx = newContext(http.MethodGet, "/user", nil)
	router.HandleFastHTTP(ctx)
	if !routed {
		t.Fatal("Routing failed!")
	}
}

func TestRouterMatchedRoutePath(t *testing.T) {
	route1 := "/user/:name"
	routed1 := false
	handle1 := func(ctx *fasthttp.RequestCtx, ps Params) {
		route := ps.MatchedRoutePath()
		if route != route1 {
			t.Fatalf("Wrong matched route: want %s, got %s", route1, route)
		}
		routed1 = true
	}

	route2 := "/user/:name/details"
	routed2 := false
	handle2 := func(ctx *fasthttp.RequestCtx, ps Params) {
		route := ps.MatchedRoutePath()
		if route != route2 {
			t.Fatalf("Wrong matched route: want %s, got %s", route2, route)
		}
		routed2 = true
	}

	route3 := "/"
	routed3 := false
	handle3 := func(ctx *fasthttp.RequestCtx, ps Params) {
		route := ps.MatchedRoutePath()
		if route != route3 {
			t.Fatalf("Wrong matched route: want %s, got %s", route3, route)
		}
		routed3 = true
	}

	router := New()
	router.SaveMatchedRoutePath = true
	router.Handle(http.MethodGet, route1, handle1)
	router.Handle(http.MethodGet, route2, handle2)
	router.Handle(http.MethodGet, route3, handle3)

	ctx := newContext(http.MethodGet, "/user/gopher", nil)
	router.HandleFastHTTP(ctx)
	if !routed1 || routed2 || routed3 {
		t.Fatal("Routing failed!")
	}

	ctx = newContext(http.MethodGet, "/user/gopher/details", nil)
	router.HandleFastHTTP(ctx)
	if !routed2 || routed3 {
		t.Fatal("Routing failed!")
	}

	ctx = newContext(http.MethodGet, "/", nil)
	router.HandleFastHTTP(ctx)
	if !routed3 {
		t.Fatal("Routing failed!")
	}
}

type mockFileSystem struct {
	opened bool
}

func (mfs *mockFileSystem) Open(name string) (http.File, error) {
	mfs.opened = true
	return nil, errors.New("this is just a mock")
}

func TestRouterServeFiles(t *testing.T) {
	router := New()
	mfs := &mockFileSystem{}

	recv := catchPanic(func() {
		router.ServeFiles("/noFilepath", mfs)
	})
	if recv == nil {
		t.Fatal("registering path not ending with '*filepath' did not panic")
	}

	router.ServeFiles("/*filepath", mfs)
	ctx := newContext(http.MethodGet, "/favicon.ico", nil)
	router.HandleFastHTTP(ctx)
	if !mfs.opened {
		t.Error("serving file failed")
	}
}

func newContext(method, url string, body io.Reader) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(url)
	ctx.Request.SetBodyStream(body, 0)
	return ctx
}
