// Copyright 2023 The LUCI Authors.
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

package permissionscfg

import (
	"context"

	"go.chromium.org/luci/auth_service/api/configspb"
	"go.chromium.org/luci/config"
	"go.chromium.org/luci/config/server/cfgcache"
)

var cachedPermissionsCfg = cfgcache.Register(&cfgcache.Entry{
	Path: "permissions.cfg",
	Type: (*configspb.PermissionsConfig)(nil),
})

// Get returns the config stored in context.
func Get(ctx context.Context) (*configspb.PermissionsConfig, error) {
	cfg, err := cachedPermissionsCfg.Get(ctx, nil)
	return cfg.(*configspb.PermissionsConfig), err
}

// SetConfig installs the cfg into the context ctx.
func SetConfig(ctx context.Context, cfg *configspb.PermissionsConfig) error {
	return cachedPermissionsCfg.Set(ctx, cfg, &config.Meta{})
}

// Update fetches the config and puts it into the datastore.
//
// It is then used by all requests that go through Middleware.
func Update(ctx context.Context) error {
	_, err := cachedPermissionsCfg.Update(ctx, nil)
	return err
}
