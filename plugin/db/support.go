package db

import "github.com/go-kratos/kratos/v2/config"

func (db *PlugDB) Name() string {
	return name
}

func (db *PlugDB) DependsOn(config.Value) []string {
	return nil
}

func (db *PlugDB) Weight() int {
	return db.weight
}

func (db *PlugDB) ConfPrefix() string {
	return confPrefix
}
