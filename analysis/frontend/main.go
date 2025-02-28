// Copyright 2022 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"net/http"

	"go.chromium.org/luci/analysis/frontend/handlers"
	"go.chromium.org/luci/analysis/internal/config"
	analysisserver "go.chromium.org/luci/analysis/server"
	"go.chromium.org/luci/auth/identity"
	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/auth"
	_ "go.chromium.org/luci/server/encryptedcookies/session/datastore"
	"go.chromium.org/luci/server/router"
	"go.chromium.org/luci/server/templates"
)

// prepareTemplates configures templates.Bundle used by all UI handlers.
func prepareTemplates(opts *server.Options) *templates.Bundle {
	return &templates.Bundle{
		Loader: templates.FileSystemLoader("templates"),
		// Controls whether templates are cached.
		DebugMode: func(context.Context) bool { return !opts.Prod },
		DefaultArgs: func(ctx context.Context, e *templates.Extra) (templates.Args, error) {
			// Login and Logout URLs take a ?r query parameter to specify
			// the redirection target after login/logout completes.
			logoutURL, err := auth.LogoutURL(ctx, "/")
			if err != nil {
				return nil, err
			}
			loginURL, err := auth.LoginURL(ctx, "/")
			if err != nil {
				return nil, err
			}

			config, err := config.Get(ctx)
			if err != nil {
				return nil, err
			}

			return templates.Args{
				"MonorailHostname": config.MonorailHostname,
				"IsAnonymous":      auth.CurrentUser(ctx).Identity.Kind() == identity.Anonymous,
				"UserName":         auth.CurrentUser(ctx).Name,
				"UserEmail":        auth.CurrentUser(ctx).Email,
				"UserAvatar":       auth.CurrentUser(ctx).Picture,
				"LogoutURL":        logoutURL,
				"LoginURL":         loginURL,
			}, nil
		},
	}
}

func pageBase(srv *server.Server) router.MiddlewareChain {
	return router.NewMiddlewareChain(
		auth.Authenticate(srv.CookieAuth),
		templates.WithTemplates(prepareTemplates(&srv.Options)),
	)
}

// Entrypoint for the default service.
func main() {
	analysisserver.Main(func(srv *server.Server) error {
		// Only the frontend service serves frontend UI. This is because
		// the frontend relies upon other assets (javascript, files) and
		// it is annoying to deploy them with every backend service.
		mw := pageBase(srv)
		handlers := handlers.NewHandlers()
		handlers.RegisterRoutes(srv.Routes, mw)
		srv.Routes.Static("/static/", mw, http.Dir("./ui/dist"))
		// Anything that is not found, serve app html and let the client side router handle it.
		srv.Routes.NotFound(mw, handlers.IndexPage)

		return nil
	})
}
