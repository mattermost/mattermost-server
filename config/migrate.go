// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"github.com/pkg/errors"
)

// Migrate migrates SAML keys, certificates, and other config files from one store to another given their data source names.
func Migrate(from, to string) error {
	source, err := NewStoreFromDSN(from, false, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to access source config %s", from)
	}
	defer source.Close()

	destination, err := NewStoreFromDSN(to, false, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to access destination config %s", to)
	}
	defer destination.Close()

	sourceConfig := source.Get()
	if _, _, err = destination.Set(sourceConfig); err != nil {
		return errors.Wrapf(err, "failed to set config")
	}

	files := []string{
		*sourceConfig.SamlSettings.IdpCertificateFile,
		*sourceConfig.SamlSettings.PublicCertificateFile,
		*sourceConfig.SamlSettings.PrivateKeyFile,
	}

	// Only migrate advanced logging config if it is not embedded JSON.
	if !isJSONMap(*sourceConfig.LogSettings.AdvancedLoggingConfig) {
		files = append(files, *sourceConfig.LogSettings.AdvancedLoggingConfig)
	}

	files = append(files, sourceConfig.PluginSettings.SignaturePublicKeyFiles...)

	for _, file := range files {
		if err := migrateFile(file, source, destination); err != nil {
			return err
		}
	}

	return nil
}

func migrateFile(name string, source *Store, destination *Store) error {
	fileExists, err := source.HasFile(name)
	if err != nil {
		return errors.Wrapf(err, "failed to check existence of %s", name)
	}

	if fileExists {
		file, err := source.GetFile(name)
		if err != nil {
			return errors.Wrapf(err, "failed to migrate %s", name)
		}
		err = destination.SetFile(name, file)
		if err != nil {
			return errors.Wrapf(err, "failed to migrate %s", name)
		}
	}

	return nil
}
