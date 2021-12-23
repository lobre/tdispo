package bow

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"time"

	"github.com/benbjohnson/hashfs"
	"github.com/justinas/alice"
	"github.com/justinas/nosurf"
)

// Default logger used in app.
var Logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

// StdChain contains a chain of middleware that can be applied to all routes.
// It gracefully handles panics to avoid spinning down the whole app.
// It logs requests and add default secure headers.
var StdChain = alice.New(
	recoverPanic,
	logRequest,
	secureHeaders,
)

// DynChain contains a chain of middleware that can be applied to all dynamic routes.
// It injects a CSRF cookie and optimizes responses for turbo frames by skipping the rendering of the layout.
var DynChain = alice.New(
	injectCSRFCookie,
	optimizeTurboFrame,
)

// ServerError writes an error message and stack trace to the errorLog,
// then sends a generic 500 Internal Server Error response to the user.
func ServerError(w http.ResponseWriter, err error) {
	trace := fmt.Sprintf("%s\n%s", err.Error(), debug.Stack())
	Logger.Output(2, trace)

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// ClientError sends a specific status code and corresponding description
// to the user. This should be used to send responses when there's a problem with the
// request that the user sent.
func ClientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// logRequest is a middleware that logs the request to the application logger.
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Logger.Printf("%s - %s %s %s", r.RemoteAddr, r.Proto, r.Method, r.URL.RequestURI())
		next.ServeHTTP(w, r)
	})
}

// recoverPanic gracefully handles any panic that happens in the current go routine.
// By default, panics don't shut the entire application (only the current go routine),
// but if one arise, the server will return an empty response. This middleware is taking
// care of recovering the panic and sending a regular 500 server error.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// make the http.Server automatically close the current connection.
				w.Header().Set("Connection", "close")

				ServerError(w, fmt.Errorf("%s", err))
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

// injectCSRFCookie injects an encrypted CSRF token in a cookie. That same token
// is used as a hidden field in forms (from nosurf.Token()).
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

// Assets handles the serving of asset files and enforces http cache by
// appending hashes to filenames.
type Assets struct {
	fsys *hashfs.FS
}

// NewAssets creates an Assets from a filesystem. It adds a convenient hashName
// function to templates that allows to retrieve the hashed filename from the filename.
// This function should be called before the parsing of views.
func NewAssets(fsys fs.FS) Assets {
	hfsys := hashfs.NewFS(fsys)
	defaultFuncs["hashName"] = hfsys.HashName
	return Assets{
		fsys: hfsys,
	}
}

// FileServer returns a handler for serving filesystem files
// taking the cache hashes into account.
func (assets Assets) FileServer() http.Handler {
	return hashfs.FileServer(assets.fsys)
}

// Run runs the http server and launches a goroutine
// to listen to os.Interrupt before stopping it gracefully.
func Run(srv *http.Server) error {
	shutdown := make(chan error)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		<-stop

		Logger.Println("shutting down server")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		shutdown <- srv.Shutdown(ctx)
	}()

	Logger.Printf("starting server on %s\n", srv.Addr)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdown
	if err != nil {
		return err
	}

	Logger.Println("server stopped")

	return nil
}
