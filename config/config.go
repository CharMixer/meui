package config

import (
  "github.com/spf13/viper"
  "strings"
)

func setDefaults() {
  viper.SetDefault("config.app.path", "./app.yml")
  viper.SetDefault("config.discovery.path", "./discovery.yml")
}

func GetInt(key string) int {
  return viper.GetInt(key)
}

func GetIntStrict(key string) int {
  return viper.GetInt(key)
}

func GetString(key string) string {
  return viper.GetString(key)
}

func GetStringStrict(key string) string {
  return viper.GetString(key)
}

func GetStringSlice(key string) []string {
  return viper.GetStringSlice(key)
}

func InitConfigurations() (error) {
  var err error

  // lets environment variable override config file
  viper.AutomaticEnv()
  viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

  setDefaults()

  // Load discovery configurations

  viper.SetConfigFile(viper.GetString("config.discovery.path"))
  err = viper.ReadInConfig() // Find and read the config file
  if err != nil { // Handle errors reading the config file
    return err
  }

  // Load app specific configurations

  viper.SetConfigFile(viper.GetString("config.app.path"))
  err = viper.MergeInConfig() // Find and read the config file
  if err != nil { // Handle errors reading the config file
    return err
  }

  return nil
}
