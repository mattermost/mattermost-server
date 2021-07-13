// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/mattermost/mattermost-server/v5/audit"
	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/shared/i18n"
	"github.com/mattermost/mattermost-server/v5/shared/mlog"
	"github.com/mattermost/mattermost-server/v5/utils"
)

const noSettingsNamed = "unable to find a setting named: %s"

var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration",
}

var ValidateConfigCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config file",
	Long:  "If the config file is valid, this command will output a success message and have a zero exit code. If it is invalid, this command will output an error and have a non-zero exit code.",
	RunE:  configValidateCmdF,
}

var ConfigSubpathCmd = &cobra.Command{
	Use:   "subpath",
	Short: "Update client asset loading to use the configured subpath",
	Long:  "Update the hard-coded production client asset paths to take into account Mattermost running on a subpath.",
	Example: `  config subpath
  config subpath --path /mattermost
  config subpath --path /`,
	RunE: configSubpathCmdF,
}

var ConfigGetCmd = &cobra.Command{
	Use:     "get",
	Short:   "Get config setting",
	Long:    "Gets the value of a config setting by its name in dot notation.",
	Example: `config get SqlSettings.DriverName`,
	Args:    cobra.ExactArgs(1),
	RunE:    configGetCmdF,
}

var ConfigShowCmd = &cobra.Command{
	Use:     "show",
	Short:   "Writes the server configuration to STDOUT",
	Long:    "Pretty-prints the server configuration and writes to STDOUT",
	Example: "config show",
	RunE:    configShowCmdF,
}

var ConfigSetCmd = &cobra.Command{
	Use:     "set",
	Short:   "Set config setting",
	Long:    "Sets the value of a config setting by its name in dot notation. Accepts multiple values for array settings",
	Example: "config set SqlSettings.DriverName mysql",
	Args:    cobra.MinimumNArgs(2),
	RunE:    configSetCmdF,
}

var MigrateConfigCmd = &cobra.Command{
	Use:     "migrate [from_config] [to_config]",
	Short:   "Migrate existing config between backends",
	Long:    "Migrate a file-based configuration to (or from) a database-based configuration. Point the Mattermost server at the target configuration to start using it",
	Example: `config migrate path/to/config.json "postgres://mmuser:mostest@localhost:5432/mattermost_test?sslmode=disable&connect_timeout=10"`,
	Args:    cobra.ExactArgs(2),
	RunE:    configMigrateCmdF,
}

var ConfigResetCmd = &cobra.Command{
	Use:     "reset",
	Short:   "Reset config setting",
	Long:    "Resets the value of a config setting by its name in dot notation or a setting section. Accepts multiple values for array settings.",
	Example: "config reset SqlSettings.DriverName LogSettings",
	RunE:    configResetCmdF,
}

func init() {
	ConfigSubpathCmd.Flags().String("path", "", "Optional subpath; defaults to value in SiteURL")
	ConfigResetCmd.Flags().Bool("confirm", false, "Confirm you really want to reset all configuration settings to its default value")
	ConfigShowCmd.Flags().Bool("json", false, "Output the configuration as JSON.")

	ConfigCmd.AddCommand(
		ValidateConfigCmd,
		ConfigSubpathCmd,
		ConfigGetCmd,
		ConfigShowCmd,
		ConfigSetCmd,
		MigrateConfigCmd,
		ConfigResetCmd,
	)
	RootCmd.AddCommand(ConfigCmd)
}

func configValidateCmdF(command *cobra.Command, args []string) error {
	utils.TranslationsPreInit()
	model.AppErrorInit(i18n.T)

	_, err := getConfigStore(command)
	if err != nil {
		return err
	}

	CommandPrettyPrintln("The document is valid")
	return nil
}

func configSubpathCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Srv().Shutdown()

	path, err := command.Flags().GetString("path")
	if err != nil {
		return errors.Wrap(err, "failed reading path")
	}

	if path == "" {
		return utils.UpdateAssetsSubpathFromConfig(a.Config())
	}

	if err := utils.UpdateAssetsSubpath(path); err != nil {
		return errors.Wrap(err, "failed to update assets subpath")
	}

	return nil
}

func getConfigStore(command *cobra.Command) (*config.Store, error) {
	if err := utils.TranslationsPreInit(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize i18n")
	}

	configStore, err := config.NewStoreFromDSN(getConfigDSN(command, config.GetEnvironment()), false, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize config store")
	}

	return configStore, nil
}

func configGetCmdF(command *cobra.Command, args []string) error {
	configStore, err := getConfigStore(command)
	if err != nil {
		return err
	}

	out, err := printConfigValues(configToMap(*configStore.Get()), strings.Split(args[0], "."), args[0])
	if err != nil {
		return err
	}

	fmt.Printf("%s", out)

	return nil
}

func configShowCmdF(command *cobra.Command, args []string) error {
	useJSON, err := command.Flags().GetBool("json")
	if err != nil {
		return errors.Wrap(err, "failed reading json parameter")
	}

	err = cobra.NoArgs(command, args)
	if err != nil {
		return err
	}

	configStore, err := getConfigStore(command)
	if err != nil {
		return err
	}

	config := *configStore.Get()

	if useJSON {
		configJSON, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			return errors.Wrap(err, "failed to marshal config as json")
		}

		fmt.Printf("%s\n", configJSON)
	} else {
		fmt.Printf("%s", prettyPrintStruct(config))
	}

	return nil
}

// printConfigValues function prints out the value of the configSettings working recursively or
// gives an error if config setting is not in the file.
func printConfigValues(configMap map[string]interface{}, configSetting []string, name string) (string, error) {
	res, ok := configMap[configSetting[0]]
	if !ok {
		return "", fmt.Errorf("%s configuration setting is not in the file", name)
	}
	value := reflect.ValueOf(res)
	switch value.Kind() {
	case reflect.Map:
		if len(configSetting) == 1 {
			return printStringMap(value, 0), nil
		}
		return printConfigValues(res.(map[string]interface{}), configSetting[1:], name)
	default:
		if len(configSetting) == 1 {
			return fmt.Sprintf("%s: \"%v\"\n", name, res), nil
		}
		return "", fmt.Errorf("%s configuration setting is not in the file", name)
	}
}

func configSetCmdF(command *cobra.Command, args []string) error {
	configStore, err := getConfigStore(command)
	if err != nil {
		return err
	}

	// args[0] -> holds the config setting that we want to change
	// args[1:] -> the new value of the config setting
	configSetting := args[0]
	newVal := args[1:]

	// create the function to update config
	oldConfig := configStore.Get().Clone()
	newConfig := configStore.Get().Clone()

	f := updateConfigValue(configSetting, newVal, oldConfig, newConfig)
	f(newConfig)

	// UpdateConfig above would have already fixed these invalid locales, but we check again
	// in the context of an explicit change to these parameters to avoid saving the fixed
	// settings in the first place.
	if changed := config.FixInvalidLocales(newConfig); changed {
		return errors.New("Invalid locale configuration")
	}

	oldCfg, newCfg, errSet := configStore.Set(newConfig)
	if errSet != nil {
		return errors.Wrap(errSet, "failed to set config")
	}

	a, errInit := InitDBCommandContextCobra(command)
	if errInit != nil {
		return errInit
	}
	defer a.Srv().Shutdown()

	auditRec := a.MakeAuditRecord("configSet", audit.Success)
	defer a.LogAuditRec(auditRec, nil)
	diffs, diffErr := config.Diff(oldCfg, newCfg)
	if diffErr != nil {
		return errors.Wrap(diffErr, "failed to diff configs")
	}
	auditRec.AddMeta("diff", diffs)

	return nil
}

func configMigrateCmdF(command *cobra.Command, args []string) error {
	from := args[0]
	to := args[1]

	err := config.Migrate(from, to)

	if err != nil {
		return errors.Wrap(err, "failed to migrate config")
	}

	mlog.Info("Successfully migrated config.")

	return nil
}

func updateConfigValue(configSetting string, newVal []string, oldConfig, newConfig *model.Config) func(*model.Config) {
	return func(update *model.Config) {

		// convert config to map[string]interface
		configMap := configToMap(*oldConfig)

		// iterate through the map and update the value or print an error and exit
		err := UpdateMap(configMap, strings.Split(configSetting, "."), newVal)
		if err != nil {
			fmt.Printf("%s\n", err)
			os.Exit(1)
		}

		// convert map to json
		bs, err := json.Marshal(configMap)
		if err != nil {
			fmt.Printf("Error while marshalling map to json %s\n", err)
			os.Exit(1)
		}

		// convert json to struct
		err = json.Unmarshal(bs, newConfig)
		if err != nil {
			fmt.Printf("Error while unmarshalling json to struct %s\n", err)
			os.Exit(1)
		}

		*update = *newConfig

	}
}

func UpdateMap(configMap map[string]interface{}, configSettings []string, newVal []string) error {
	res, ok := configMap[configSettings[0]]
	if !ok {
		return fmt.Errorf(noSettingsNamed, configSettings[0])
	}

	value := reflect.ValueOf(res)

	switch value.Kind() {

	case reflect.Map:
		// we can only change the value of a particular setting, not the whole map, return error
		if len(configSettings) == 1 {
			return errors.New("unable to set multiple settings at once")
		}
		simpleMap, ok := res.(map[string]interface{})
		if ok {
			return UpdateMap(simpleMap, configSettings[1:], newVal)
		}
		mapOfTheMap, ok := res.(map[string]map[string]interface{})
		if ok {
			convertedMap := make(map[string]interface{})
			for k, v := range mapOfTheMap {
				convertedMap[k] = v
			}
			return UpdateMap(convertedMap, configSettings[1:], newVal)
		}
		pluginStateMap, ok := res.(map[string]*model.PluginState)
		if ok {
			convertedMap := make(map[string]interface{})
			for k, v := range pluginStateMap {
				convertedMap[k] = v
			}
			return UpdateMap(convertedMap, configSettings[1:], newVal)
		}
		return fmt.Errorf(noSettingsNamed, configSettings[1])

	case reflect.Int:
		if len(configSettings) == 1 {
			val, err := strconv.Atoi(newVal[0])
			if err != nil {
				return err
			}
			configMap[configSettings[0]] = val
			return nil
		}
		return fmt.Errorf(noSettingsNamed, configSettings[0])

	case reflect.Int64:
		if len(configSettings) == 1 {
			val, err := strconv.Atoi(newVal[0])
			if err != nil {
				return err
			}
			configMap[configSettings[0]] = int64(val)
			return nil
		}
		return fmt.Errorf(noSettingsNamed, configSettings[0])

	case reflect.Bool:
		if len(configSettings) == 1 {
			val, err := strconv.ParseBool(newVal[0])
			if err != nil {
				return err
			}
			configMap[configSettings[0]] = val
			return nil
		}
		return fmt.Errorf(noSettingsNamed, configSettings[0])

	case reflect.String:
		if len(configSettings) == 1 {
			configMap[configSettings[0]] = newVal[0]
			return nil
		}
		return fmt.Errorf(noSettingsNamed, configSettings[0])

	case reflect.Slice:
		if len(configSettings) == 1 {
			configMap[configSettings[0]] = newVal
			return nil
		}
		return fmt.Errorf(noSettingsNamed, configSettings[0])

	case reflect.Ptr:
		state, ok := res.(*model.PluginState)
		if !ok || len(configSettings) != 2 {
			return errors.New("type not supported yet")
		}
		val, err := strconv.ParseBool(newVal[0])
		if err != nil {
			return err
		}
		state.Enable = val
		return nil

	default:
		return errors.New("type not supported yet")
	}
}

func configResetCmdF(command *cobra.Command, args []string) error {
	configStore, err := getConfigStore(command)
	if err != nil {
		return err
	}

	a, errInit := InitDBCommandContextCobra(command)
	if errInit != nil {
		return errInit
	}
	defer a.Srv().Shutdown()

	var oldCfg *model.Config
	var newCfg *model.Config

	defer func() {
		auditRec := a.MakeAuditRecord("configReset", audit.Success)
		if oldCfg != nil && newCfg != nil {
			diffs, diffErr := config.Diff(oldCfg, newCfg)
			if diffErr != nil {
				mlog.Warn("Failed to diff configs", mlog.Err(diffErr))
				return
			}
			auditRec.AddMeta("diff", diffs)
		}
		a.LogAuditRec(auditRec, nil)
	}()

	defaultConfig := &model.Config{}
	defaultConfig.SetDefaults()

	confirmFlag, _ := command.Flags().GetBool("confirm")
	if confirmFlag {
		if oldCfg, newCfg, err = configStore.Set(defaultConfig); err != nil {
			return errors.Wrap(err, "failed to set config")
		}
		return nil
	}

	if !confirmFlag && len(args) == 0 {
		var confirmResetAll string
		CommandPrettyPrintln("Are you sure you want to reset all the configuration settings?(YES/NO): ")
		fmt.Scanln(&confirmResetAll)
		if confirmResetAll == "YES" {
			if oldCfg, newCfg, err = configStore.Set(defaultConfig); err != nil {
				return errors.Wrap(err, "failed to set config")
			}
		}
		return nil
	}

	tempConfig := configStore.Get().Clone()
	tempConfigMap := configToMap(*tempConfig)
	defaultConfigMap := configToMap(*defaultConfig)
	for _, arg := range args {
		err = changeMap(tempConfigMap, defaultConfigMap, strings.Split(arg, "."))
		if err != nil {
			return errors.Wrap(err, "Failed to reset config")
		}
	}
	bs, err := json.Marshal(tempConfigMap)
	if err != nil {
		fmt.Printf("Error while marshalling map to json %s\n", err)
		os.Exit(1)
	}
	err = json.Unmarshal(bs, tempConfig)
	if err != nil {
		fmt.Printf("Error while unmarshalling json to struct %s\n", err)
		os.Exit(1)
	}
	if changed := config.FixInvalidLocales(tempConfig); changed {
		return errors.New("Invalid locale configuration")
	}

	oldCfg, newCfg, err = configStore.Set(tempConfig)
	if err != nil {
		return errors.Wrap(err, "failed to set config")
	}

	return nil
}

func changeMap(oldConfigMap, defaultConfigMap map[string]interface{}, configSettings []string) error {
	resOld, ok := oldConfigMap[configSettings[0]]
	if !ok {
		return fmt.Errorf("Unable to find a setting with that name %s", configSettings[0])
	}
	resDef := defaultConfigMap[configSettings[0]]
	valueOld := reflect.ValueOf(resOld)

	if valueOld.Kind() == reflect.Map {
		if len(configSettings) == 1 {
			return changeSection(resOld.(map[string]interface{}), resDef.(map[string]interface{}))
		}
		return changeMap(resOld.(map[string]interface{}), resDef.(map[string]interface{}), configSettings[1:])
	}
	if len(configSettings) == 1 {
		oldConfigMap[configSettings[0]] = defaultConfigMap[configSettings[0]]
		return nil
	}
	return fmt.Errorf("Unable to find a setting with that name %s", configSettings[0])
}

func changeSection(oldConfigMap, defaultConfigMap map[string]interface{}) error {
	valueOld := reflect.ValueOf(oldConfigMap)
	for _, key := range valueOld.MapKeys() {
		oldConfigMap[key.String()] = defaultConfigMap[key.String()]
	}
	return nil
}

// configToMap converts our config into a map
func configToMap(s interface{}) map[string]interface{} {
	return structToMap(s)
}
