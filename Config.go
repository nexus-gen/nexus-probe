package main

type Target struct {
	URL      string `yaml:"url"`
	Category string `yaml:"category"`
}
type Config struct {
	Targets []Target `yaml:"targets"`
}
