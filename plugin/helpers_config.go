// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package plugin

import (
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

// CheckRequiredServerConfiguration implements Helpers.CheckRequiredServerConfiguration
func (p *HelpersImpl) CheckRequiredServerConfiguration(req *model.Config) (bool, error) {
	if req == nil {
		return true, nil
	}

	cfg := p.API.GetConfig()

	mc, err := utils.Merge(cfg, req, nil)
	if err != nil {
		return false, errors.Wrap(err, "could not merge configurations")
	}

	mergedCfg := mc.(model.Config)
	if mergedCfg.ToJSON() != cfg.ToJSON() {
		return false, nil
	}

	return true, nil
}
