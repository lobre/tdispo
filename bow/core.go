package bow

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/benbjohnson/hashfs"
	"github.com/golangcollege/sessions"
	"github.com/justinas/alice"
	"github.com/justinas/nosurf"
)

// Core holds the core logic to configure and run a simple web app.
// It is meant to be embedded in a parent web app structure.
type Core struct {
	logger *log.Logger
	fsys   fs.FS
	hfsys  *hashfs.FS

	db      *DB
	views   *views
	session *sessions.Session
}

// NewCore creates a core with sane defaults. Options can be used for specific configurations.
func NewCore(fsys fs.FS, options ...Option) (*Core, error) {
	hfsys := hashfs.NewFS(fsys)

	core := &Core{
		logger: log.New(os.Stdout, "", log.Ldate|log.Ltime),

		fsys:  fsys,
		hfsys: hfsys,

		views: &views{
			pages:    make(map[string]*template.Template),
			partials: make(map[string]*template.Template),
			funcs:    make(template.FuncMap),
			reqFuncs: make(ReqFuncMap),
		},
	}

	for _, opt := range options {
		if err := opt(core); err != nil {
			return nil, err
		}
	}

	core.views.logger = core.logger

	// set default funcs
	core.views.funcs["hash"] = hfsys.HashName
	core.views.funcs["safe"] = safe

	// set default req funcs
	core.views.reqFuncs["partial"] = core.views.partial
	core.views.reqFuncs["flash"] = func(r *http.Request) interface{} {
		return func() string {
			return core.session.PopString(r, "flash")
		}
	}
	core.views.reqFuncs["csrf"] = func(r *http.Request) interface{} {
		return func() string {
			return nosurf.Token(r)
		}
	}

	if err := core.views.Parse(fsys); err != nil {
		return nil, err
	}

	return core, nil
}

// Option configures an core.
type Option func(*Core) error

// WithLogger is an option to set the application logger.
func WithLogger(logger *log.Logger) Option {
	return func(core *Core) error {
		core.logger = logger
		return nil
	}
}

// WithDB is an option to enable and configure the database access.
func WithDB(dsn string) Option {
	return func(core *Core) error {
		core.db = NewDB(dsn, core.fsys)
		if err := core.db.Open(); err != nil {
			return err
		}
		return nil
	}
}

// WithSession is an option to enable cookie sessions.
// The key parameter is the secret you want to use to authenticate
// and encrypt sessions cookies, and should be 32 bytes long.
func WithSession(key string) Option {
	return func(core *Core) error {
		core.session = sessions.New([]byte(key))
		core.session.Lifetime = 12 * time.Hour
		return nil
	}
}

// WithFuncs is an option to configure default functions that will
// be injected into views.
func WithFuncs(funcs template.FuncMap) Option {
	return func(core *Core) error {
		for k, fn := range funcs {
			core.views.funcs[k] = fn
		}
		return nil
	}
}

// WithReqFuncs is an option similar to WithFuncs, but with functions that
// are request-aware.
func WithReqFuncs(funcs ReqFuncMap) Option {
	return func(core *Core) error {
		for k, fn := range funcs {
			core.views.reqFuncs[k] = fn
		}
		return nil
	}
}

// WithGlobals is an option that allows to define a function that is
// called at each rendering to inject data that can be retrieved using the
// "globals" helper template function.
func WithGlobals(fn func(*http.Request) interface{}) Option {
	return func(core *Core) error {
		core.views.reqFuncs["globals"] = func(r *http.Request) interface{} {
			return func() interface{} {
				return fn(r)
			}
		}
		return nil
	}
}

// FileServer returns a handler for serving filesystem files.
// It enforces http cache by appending hashes to filenames.
// A hashName function is defined in templates to gather the hashed filename of a file.
func (core *Core) FileServer() http.Handler {
	return hashfs.FileServer(core.hfsys)
}

// Log returns the application logger.
func (core *Core) Log() *log.Logger {
	return core.logger
}

// Session returns the application session.
func (core *Core) Session() *sessions.Session {
	return core.session
}

// StdChain returns a chain of middleware that can be applied to all routes.
// It gracefully handles panics to avoid spinning down the whole app.
// It logs requests and add default secure headers.
func (core *Core) StdChain() alice.Chain {
	return alice.New(
		core.recoverPanic,
		core.logRequest,
		secureHeaders,
	)
}

// DynChain returns a chain of middleware that can be applied to all dynamic routes.
// It injects a CSRF cookie and enable sessions.
func (core *Core) DynChain() alice.Chain {
	chain := alice.New(injectCSRFCookie)
	if core.session != nil {
		chain = chain.Append(core.session.Enable)
	}
	return chain
}

// logRequest is a middleware that logs the request to the application logger.
func (core *Core) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		core.logger.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}

// recoverPanic is a middleware that gracefully handles any panic that happens in the
// current go routine.
// By default, panics don't shut the entire application (only the current go routine),
// but if one arise, the server will return an empty response. This middleware is taking
// care of recovering the panic and sending a regular 500 server error.
func (core *Core) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// make the http.Server automatically close the current connection.
				w.Header().Set("Connection", "close")
				core.ServerError(w, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// secureHeaders is a middleware that injects headers in the response
// to prevent XSS and Clickjacking attacks.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Frame-Options", "deny")

		next.ServeHTTP(w, r)
	})
}

// injectCSRFCookie is a middleware that injects an encrypted CSRF token in a cookie.
// That same token is used as a hidden field in forms (from nosurf.Token()).
// On the form submission, the server checks that these two values match.
// So directly trying to post a request to our secured endpoint without this parameter would fail.
// The only way to submit the form is from our frontend.
func injectCSRFCookie(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
	})

	return csrfHandler
}

// ServerError is a convenient wrapper around views.ServerError.
func (core *Core) ServerError(w http.ResponseWriter, err error) {
	core.views.ServerError(w, err)
}

// ClientError is a convenient wrapper around views.ClientError.
func (core *Core) ClientError(w http.ResponseWriter, status int) {
	core.views.ClientError(w, status)
}

// Render is a convenient wrapper around views.Render.
func (core *Core) Render(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	core.views.Render(w, r, name, data)
}

// RenderStream is a convenient wrapper around views.RenderStream.
func (core *Core) RenderStream(action StreamAction, target string, w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	core.views.RenderStream(action, target, w, r, name, data)
}

// Flash sets a flash message to the session.
func (core *Core) Flash(r *http.Request, msg string) {
	core.session.Put(r, "flash", msg)
}

// Run runs the http server and launches a goroutine
// to listen to os.Interrupt before stopping it gracefully.
func (core *Core) Run(srv *http.Server) error {
	shutdown := make(chan error)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop

		core.logger.Println("shutting down server")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		shutdown <- srv.Shutdown(ctx)
	}()

	core.logger.Printf("starting server on %s\n", srv.Addr)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdown
	if err != nil {
		return err
	}

	core.logger.Println("server stopped")

	return nil
}
