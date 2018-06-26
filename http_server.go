package phttp

import (
    "fmt"
    "net/http"
    "context"
    "time"
)
const (
    defaultShutdownInterval         = 3
)

type HTTPWorker struct {
    addr        string
    srv         *http.Server
    static      *static
    router
    logFunc     func(format string, args ...interface{})
}

func New(addr string) *HTTPWorker {
    w := &HTTPWorker{addr: addr}
    w.initRouter()
    return w
}

func (w *HTTPWorker) Serve() error {
    w.mergeRoute()

    w.srv = &http.Server{
        Addr: w.addr,
        Handler: w,
        //ReadTimeout:    a.Conf.App.ReadTimeout,
        //WriteTimeout:   a.Conf.App.WriteTimeout,
        //MaxHeaderBytes: a.Conf.App.MaxHeaderBytes,
    }
    if w.logFunc != nil {
        w.logFunc("[http] start listening on %v.", w.srv.Addr)
    }

    go func() {
        if err := w.srv.ListenAndServe(); err != nil {
            if err != http.ErrServerClosed {
                if w.logFunc != nil {
                    w.logFunc("[http] ListenAndServe() error %v.", err)
                } else {
                    fmt.Printf("Httpserver: ListenAndServe() error: %s\n", err.Error())
                }
            }
        }
    }()

    return nil
}

func (w *HTTPWorker) Close() {
    if w.logFunc != nil {
        w.logFunc("[http] stop listening on %v.", w.srv.Addr)
    }
    ctx, _ := context.WithTimeout(context.Background(), defaultShutdownInterval * time.Second)
    w.srv.Shutdown(ctx)
}

func (w *HTTPWorker) SetLog(l func(format string, args ...interface{})) {
    w.logFunc = l
}

//实现ServeMux接口
func (w *HTTPWorker) ServeHTTP(writer http.ResponseWriter, r *http.Request) {
    //todo recover

    //makeContext
    ctx := makeContext(writer, r)
    request := ctx.Request()
    response := ctx.Response()

    defer response.flush()

    //length check

    //static handler
    if w.static != nil {
        file := w.static.match(request.Path())
        if file != "" {
            //todo 是否启用全局middleware?
            response.File(file)
            return
        }
    }

    //router handler
    route := w.match(request.Method(), request.Path())
    if route == nil {
        if w.logFunc != nil {
            w.logFunc("[http] route not found, %v", request.Path())
        }
        var notfound handler = func(context *Context) error {
            context.Response().Error(http.StatusNotFound, "invalid path or method")
            return nil
        }
        fnChain := w.makeHandlers([]appliable{notfound})
        fnChain(ctx)
        return
    }
    if w.logFunc != nil {
        w.logFunc("[http] route found ok, %v", request.Path())
    }
    route.fnChain(ctx)
}

func (w *HTTPWorker) Static(prefix, dir string) error {
    if w.static == nil {
        w.static = &static{}
    }
    return w.static.serve(prefix, dir)
}
