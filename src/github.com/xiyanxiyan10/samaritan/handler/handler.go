package handler

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/hprose/hprose-golang/rpc"
	"github.com/gorilla/sessions"
	"github.com/xiyanxiyan10/samaritan/config"
	"github.com/xiyanxiyan10/samaritan/constant"

)

type response struct {
	Success bool
	Message string
	Data    interface{}
}

type event struct{}

func (e event) OnSendHeader(ctx *rpc.HTTPContext) {
	ctx.Response.Header().Set("Access-Control-Allow-Headers", "Authorization")
}

// Server ...
func Server() {
	var store = sessions.NewCookieStore([]byte("session_cookie"))
	port := config.String("port")
	service := rpc.NewHTTPService()
	handler := struct {
		User      user
		Exchange  exchange
		Algorithm algorithm
		Trader    runner
		Log       logger
	}{}
	service.Event = event{}
	service.AddBeforeFilterHandler(func(request []byte, ctx rpc.Context, next rpc.NextFilterHandler) (response []byte, err error) {

		ctx.SetInt64("start", time.Now().UnixNano())
		httpContext := ctx.(*rpc.HTTPContext)

		if httpContext != nil {
			//@Todo session filter here, entry set to login address
			session, _ := store.Get(httpContext.Request, "session-name")
			// Set some session values.
			session.Values["foo"] = "bar"
			session.Values[42] = 43
			// Save it before we write to the response/return from the handler.
			session.Save(httpContext.Request, httpContext.Response)
			ctx.SetString("username", parseToken(httpContext.Request.Header.Get("Authorization")))
		}
		return next(request, ctx)
	})
	service.AddInvokeHandler(func(name string, args []reflect.Value, ctx rpc.Context, next rpc.NextInvokeHandler) (results []reflect.Value, err error) {
		name = strings.Replace(name, "_", ".", 1)
		results, err = next(name, args, ctx)
		spend := (time.Now().UnixNano() - ctx.GetInt64("start")) / 1000000
		spendInfo := ""
		if spend > 1000 {
			spendInfo = fmt.Sprintf("%vs", spend/1000)
		} else {
			spendInfo = fmt.Sprintf("%vms", spend)
		}
		log.Printf("%16s() spend %s", name, spendInfo)
		return
	})
	service.AddAllMethods(handler)
	http.Handle("/api", service)
	http.Handle("/", http.FileServer(http.Dir( config.String("webdist"))))
	fmt.Printf("%v  Version %v\n", constant.Banner, constant.Version)
	log.Printf("Running at http://localhost:%v\n", port)
	http.ListenAndServe(":"+port, nil)
}
